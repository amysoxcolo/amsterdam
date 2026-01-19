/*
 * Amsterdam Web Communities System
 * Copyright (c) 2025-2026 Erbosoft Metaverse Design Solutions, All Rights Reserved
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */
// Package ui holds the support for the Amsterdam user interface, wrapping Echo and Jet templates.
package ui

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

type AmSessionOptions struct {
	Path        string
	Domain      string
	MaxAge      int
	Secure      bool
	HttpOnly    bool
	Partitioned bool
	SameSite    http.SameSite
}

func newCookieFromOptions(name, value string, options *AmSessionOptions) *http.Cookie {
	return &http.Cookie{
		Name:        name,
		Value:       value,
		Path:        options.Path,
		Domain:      options.Domain,
		MaxAge:      options.MaxAge,
		Secure:      options.Secure,
		HttpOnly:    options.HttpOnly,
		Partitioned: options.Partitioned,
		SameSite:    options.SameSite,
	}
}

type AmSession interface {
	ID() string
	Name() string
	Save(*http.Request, http.ResponseWriter) error
	Store() AmSessionStore
	Options() *AmSessionOptions
	IsNew() bool
	SetNew(bool)
	AddFlash(value any, vars ...string)
	Flashes(vars ...string) []any
	Get(any) (any, bool)
	Set(any, any)
	Erase()
}

type AmSessionStore interface {
	Get(*http.Request, string) (AmSession, error)
	New(*http.Request, string) (AmSession, error)
	Save(*http.Request, http.ResponseWriter, AmSession) error
	SessionInfo() (int, []string, int)
}

type amSession struct {
	mutex   sync.RWMutex
	id      string
	values  map[any]any
	options *AmSessionOptions
	isNew   bool
	store   AmSessionStore
	name    string
}

const defaultFlashKey = "__flash"

func (sess *amSession) ID() string {
	return sess.id
}

func (sess *amSession) Name() string {
	return sess.name
}

func (sess *amSession) Save(r *http.Request, w http.ResponseWriter) error {
	return sess.store.Save(r, w, sess)
}

func (sess *amSession) Store() AmSessionStore {
	return sess.store
}

func (sess *amSession) Options() *AmSessionOptions {
	return sess.options
}

func (sess *amSession) IsNew() bool {
	return sess.isNew
}

func (sess *amSession) SetNew(v bool) {
	sess.mutex.Lock()
	sess.isNew = v
	sess.mutex.Unlock()
}

func (sess *amSession) AddFlash(value any, vars ...string) {
	key := defaultFlashKey
	if len(vars) > 0 {
		key = vars[0]
	}
	var flashes []any
	sess.mutex.Lock()
	defer sess.mutex.Unlock()
	if v, ok := sess.values[key]; ok {
		flashes = v.([]any)
	}
	sess.values[key] = append(flashes, value)
}

func (sess *amSession) Flashes(vars ...string) []any {
	var flashes []any
	key := defaultFlashKey
	if len(vars) > 0 {
		key = vars[0]
	}
	sess.mutex.Lock()
	defer sess.mutex.Unlock()
	if v, ok := sess.values[key]; ok {
		delete(sess.values, key)
		flashes = v.([]any)
	}
	return flashes
}

func (sess *amSession) Get(key any) (any, bool) {
	sess.mutex.RLock()
	defer sess.mutex.RUnlock()
	v, ok := sess.values[key]
	return v, ok
}

func (sess *amSession) Set(key, value any) {
	sess.mutex.Lock()
	defer sess.mutex.Unlock()
	sess.values[key] = value
}

func (sess *amSession) Erase() {
	sess.mutex.Lock()
	defer sess.mutex.Unlock()
	for k := range sess.values {
		delete(sess.values, k)
	}
}

type amSessionStore struct {
	mutex        sync.RWMutex
	sessions     map[string]*amSession
	maxEntries   int
	expiry       time.Duration
	sweepRunning atomic.Bool
}

func createAmSessionStore(exp time.Duration) *amSessionStore {
	rc := &amSessionStore{
		sessions:   make(map[string]*amSession),
		maxEntries: 0,
		expiry:     exp,
	}
	rc.sweepRunning.Store(true)
	return rc
}

func (st *amSessionStore) Get(r *http.Request, name string) (AmSession, error) {
	cookie, err := r.Cookie(name)
	if err == nil {
		st.mutex.RLock()
		session, ok := st.sessions[cookie.Value]
		if ok {
			session.isNew = false
		}
		st.mutex.RUnlock()
		if ok {
			return session, nil
		}
	}
	return st.New(r, name)
}

func (st *amSessionStore) New(r *http.Request, name string) (AmSession, error) {
	session := &amSession{
		values:  make(map[any]any),
		options: new(AmSessionOptions),
		isNew:   true,
		store:   st,
		name:    name,
	}
	idBytes := make([]byte, 32)
	if _, err := rand.Read(idBytes); err != nil {
		return nil, err
	}
	session.id = hex.EncodeToString(idBytes)
	st.mutex.Lock()
	st.sessions[session.id] = session
	if len(st.sessions) > st.maxEntries {
		st.maxEntries = len(st.sessions)
	}
	st.mutex.Unlock()
	return session, nil
}

func (st *amSessionStore) Save(r *http.Request, w http.ResponseWriter, sess AmSession) error {
	cookie := newCookieFromOptions(sess.Name(), sess.ID(), sess.Options())
	if sess.Options().MaxAge > 0 {
		d := time.Duration(sess.Options().MaxAge) * time.Second
		cookie.Expires = time.Now().Add(d)
	} else if sess.Options().MaxAge < 0 {
		cookie.Expires = time.Unix(1, 0)
	}
	http.SetCookie(w, cookie)
	return nil
}

func (st *amSessionStore) SessionInfo() (int, []string, int) {
	anons := 0
	users := make([]string, 0, len(st.sessions))
	st.mutex.RLock()
	for _, s := range st.sessions {
		v, ok := s.Get("user_anon")
		if ok && v.(bool) {
			anons++
		} else {
			name, _ := s.Get("user_name")
			users = append(users, name.(string))
		}
	}
	st.mutex.RUnlock()
	slices.Sort(users)
	return anons, users, st.maxEntries
}

/* sweep sweeps sessions to remove expired ones.
 * Parameters:
 *     tick - Channel that "pulses" periodically to run the task.
 *     done - Channel we write to when we're done.
 */
func (st *amSessionStore) sweep(tick <-chan time.Time, done chan bool) {
	for range tick {
		if st.sweepRunning.Load() {
			// phase 1 - identify expired sessions
			st.mutex.RLock()
			zap := make([]string, 0, len(st.sessions))
			for k, v := range st.sessions {
				lastTime, ok := v.Get("lasthit")
				if ok && time.Since(lastTime.(time.Time)) > st.expiry {
					zap = append(zap, k)
				}
			}
			st.mutex.RUnlock()

			// phase 2 - get rid of the expired sessions
			for _, k := range zap {
				st.mutex.Lock()
				s, ok := st.sessions[k]
				if ok {
					delete(st.sessions, k)
					s.Erase()
				}
				st.mutex.Unlock()
			}
		} else {
			break
		}
	}
	done <- true
}

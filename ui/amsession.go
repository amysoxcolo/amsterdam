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
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"git.erbosoft.com/amy/amsterdam/config"
	"git.erbosoft.com/amy/amsterdam/database"
	"github.com/labstack/echo/v4"
	log "github.com/sirupsen/logrus"
)

/*
	This is mainly a rewrite of parts of Gorilla Sessions, but with a more-defined session interface so that we can mutex-protect
	the session variables, as our use case also dictates that the sessions be part of a global map in the session store so they can
	be timed out as well as used to show the logged-in users.  This is similar to the session support provided in J2EE servlets.
*/

// AmSessionOptions gives the options for the session.
type AmSessionOptions struct {
	Path        string
	Domain      string
	MaxAge      int
	Secure      bool
	HttpOnly    bool
	Partitioned bool
	SameSite    http.SameSite
}

// newCookieFromOptions creates a new HTTP cookie given the options.
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

// AmSession is the public session interface.
type AmSession interface {
	ID() string
	Name() string
	Save(*http.Request, http.ResponseWriter) error
	Store() AmSessionStore
	Options() *AmSessionOptions
	SetOptions(*AmSessionOptions)
	IsNew() bool
	SetNew(bool)
	AddFlash(value any, vars ...string)
	Flashes(vars ...string) []any
	Get(any) (any, bool)
	Set(any, any)
	Erase()
	Uid() (int32, bool)
	SetUser(*database.User)
	FirstTime(context.Context)
	Reset(context.Context)
	Hit()
}

// AmSessionStore is the public interface to the session store.
type AmSessionStore interface {
	Get(*http.Request, string) (AmSession, error)
	New(*http.Request, string) (AmSession, error)
	Save(*http.Request, http.ResponseWriter, AmSession) error
	SessionInfo() (int, []string, int)
}

// amSession is the implementation structure for AmSession.
type amSession struct {
	mutex   sync.RWMutex
	id      string
	values  map[any]any
	options *AmSessionOptions
	isNew   bool
	store   AmSessionStore
	name    string
}

// defaultFlashKey is the default sesison variable key for "flashes."
const defaultFlashKey = "__flash"

// ID returns the ID of the session.
func (sess *amSession) ID() string {
	return sess.id
}

// Name returns the name of the session, used for the cookie name.
func (sess *amSession) Name() string {
	return sess.name
}

// Save is a helper function that calls the session store to save this session.
func (sess *amSession) Save(r *http.Request, w http.ResponseWriter) error {
	return sess.store.Save(r, w, sess)
}

// Store returns the pointer to the session store.
func (sess *amSession) Store() AmSessionStore {
	return sess.store
}

// Options returns the options for this session.
func (sess *amSession) Options() *AmSessionOptions {
	return sess.options
}

func (sess *amSession) SetOptions(opt *AmSessionOptions) {
	sess.options = opt
}

// IsNew returns the "new" flag of this session.
func (sess *amSession) IsNew() bool {
	return sess.isNew
}

// SetNew sets the "new" flag of this session.
func (sess *amSession) SetNew(v bool) {
	sess.mutex.Lock()
	sess.isNew = v
	sess.mutex.Unlock()
}

// AddFlash adds a "flash message" to the session. The second parameter allows optionally specifying the variable name.
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

// Flashes retrueves all "flash messages" from the session. The second parameter allows optionally specifying the variable name.
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

// Get gets a session variable.
func (sess *amSession) Get(key any) (any, bool) {
	sess.mutex.RLock()
	defer sess.mutex.RUnlock()
	v, ok := sess.values[key]
	return v, ok
}

// Set sets a session variable.
func (sess *amSession) Set(key, value any) {
	sess.mutex.Lock()
	defer sess.mutex.Unlock()
	sess.values[key] = value
}

// Erase erases all session variables.
func (sess *amSession) Erase() {
	sess.mutex.Lock()
	defer sess.mutex.Unlock()
	for k := range sess.values {
		delete(sess.values, k)
	}
}

// Uid returns the current user ID associated with this session.
func (sess *amSession) Uid() (int32, bool) {
	if rc, ok := sess.Get("user_id"); ok {
		return rc.(int32), ok
	}
	return -1, false
}

// SetUser sets a user into the session, saving off the username, ID, and anonymous flag.
func (sess *amSession) SetUser(user *database.User) {
	sess.mutex.Lock()
	defer sess.mutex.Unlock()
	sess.values["user_id"] = user.Uid
	sess.values["user_name"] = user.Username
	sess.values["user_anon"] = user.IsAnon
}

// setAnon sets this session to contain the anonymous user.
func (sess *amSession) setAnon(ctx context.Context) {
	u, err := database.AmGetAnonUser(ctx)
	if err == nil {
		sess.SetUser(u)
	} else {
		log.Errorf("unable to set anonymous user: %v", err)
	}
}

// FirstTime prepares the session after it was just created.
func (sess *amSession) FirstTime(ctx context.Context) {
	sess.setAnon(ctx)
	sess.Set("lasthit", time.Now())
}

// Reset resets a session after it's been timed out.
func (sess *amSession) Reset(ctx context.Context) {
	sess.Erase()
	sess.setAnon(ctx)
	sess.Set("lasthit", time.Now())
}

// Hit updates the last-hit time in the session.
func (sess *amSession) Hit() {
	sess.Set("lasthit", time.Now())
}

// amSessionStore is the implementatiuon structure for AmSessionStore.
type amSessionStore struct {
	mutex        sync.RWMutex
	sessions     map[string]*amSession
	maxEntries   int
	expiry       time.Duration
	sweepRunning atomic.Bool
}

// createAmSessionStore creates the session store.
func createAmSessionStore(exp time.Duration) *amSessionStore {
	rc := &amSessionStore{
		sessions:   make(map[string]*amSession),
		maxEntries: 0,
		expiry:     exp,
	}
	rc.sweepRunning.Store(true)
	return rc
}

// Get retrieves a session from the request cookie.
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

// New creates a new session.
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

// Save saves the session identifier to the response cookies.
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

// SessionInfo returns the number of anonymous sessions, all the session user names, and the current maximum number of sessions.
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

// sessionStore is the global session store.
var sessionStore *amSessionStore

// setupSessionManager sets up the session store and its sweeper goroutine.
func setupSessionManager() func() {
	// get the time for the session to expire
	d, err := time.ParseDuration(config.GlobalConfig.Site.SessionExpire)
	if err != nil {
		d, err = time.ParseDuration("1h")
		if err != nil {
			panic(err.Error())
		}
	}

	// create session store
	sessionStore = createAmSessionStore(d)

	// get the clock value to run sweeps
	d, err = time.ParseDuration("1s")
	if err != nil {
		panic(err.Error())
	}

	// set up the sweep runner
	tkr := time.NewTicker(d)
	done := make(chan bool)
	go sessionStore.sweep(tkr.C, done)
	return func() {
		// stop the sweep runner
		sessionStore.sweepRunning.Store(false)
		<-done
		tkr.Stop()
	}
}

// SessionStoreInjector is middleware that injects the session store into the context variables.
func SessionStoreInjector(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		c.Set("AmSessionStore", sessionStore)
		return next(c)
	}
}

// AmSessions returns the information about all current sessions.
func AmSessions() (int, []string, int) {
	return sessionStore.SessionInfo()
}

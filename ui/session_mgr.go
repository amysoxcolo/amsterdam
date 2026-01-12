/*
 * Amsterdam Web Communities System
 * Copyright (c) 2025 Erbosoft Metaverse Design Solutions, All Rights Reserved
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
	"encoding/gob"
	"encoding/hex"
	"net/http"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"git.erbosoft.com/amy/amsterdam/config"
	"git.erbosoft.com/amy/amsterdam/database"
	"github.com/gorilla/sessions"
	log "github.com/sirupsen/logrus"
)

func AmSessionGet(sess *sessions.Session, key any) (any, bool) {
	if sess == nil {
		return 0, false
	}
	mtx := sess.Values["_mutex"].(*sync.RWMutex)
	mtx.RLock()
	defer mtx.RUnlock()
	rc, ok := sess.Values[key]
	return rc, ok
}

func AmSessionPut(sess *sessions.Session, key, value any) {
	if sess != nil {
		mtx := sess.Values["_mutex"].(*sync.RWMutex)
		mtx.Lock()
		defer mtx.Unlock()
		sess.Values[key] = value
	}
}

func AmSessionErase(sess *sessions.Session) {
	if sess != nil {
		mtx := sess.Values["_mutex"].(*sync.RWMutex)
		mtx.Lock()
		defer mtx.Unlock()
		for k := range sess.Values {
			if k != "_mutex" {
				delete(sess.Values, k)
			}
		}
	}
}

// AmsterdamStore is our implewmentation of the Gorilla session store that works close to HttpSession in Java.
type AmsterdamStore struct {
	mutex        sync.RWMutex
	sessions     map[string]*sessions.Session
	maxEntries   int
	expiry       time.Duration
	sweepRunning atomic.Bool
}

func createAmsterdamStore(exp time.Duration) *AmsterdamStore {
	rc := AmsterdamStore{
		sessions:   make(map[string]*sessions.Session),
		maxEntries: 0,
		expiry:     exp,
	}
	rc.sweepRunning.Store(true)
	return &rc
}

/* Get (from Store interface) retrieves a new or existing session for the request.
 * Parameters:
 *     r - The HTTP request object.
 *     name - The name of the session.
 * Returns:
 *     Session pointer (new or existing)
 *     Standard Go error status.
 */
func (st *AmsterdamStore) Get(r *http.Request, name string) (*sessions.Session, error) {
	cookie, err := r.Cookie(name)
	if err == nil {
		st.mutex.RLock()
		session, ok := st.sessions[cookie.Value]
		if ok {
			session.IsNew = false
		}
		st.mutex.RUnlock()
		if ok {
			return session, nil
		}
	}
	return st.New(r, name)
}

/* New (from Store interface) creates and returns a new session object.
 * Parameters:
 *     r - The HTTP request object.
 *     name - The name of the session.
 * Returns:
 *     New session pointer
 *     Standard Go error status.
 */
func (st *AmsterdamStore) New(r *http.Request, name string) (*sessions.Session, error) {
	session := sessions.NewSession(st, name)
	session.IsNew = true
	session.Values["_mutex"] = new(sync.RWMutex)
	idBytes := make([]byte, 32)
	if _, err := rand.Read(idBytes); err != nil {
		return nil, err
	}
	session.ID = hex.EncodeToString(idBytes)
	st.mutex.Lock()
	st.sessions[session.ID] = session
	if len(st.sessions) > st.maxEntries {
		st.maxEntries = len(st.sessions)
	}
	st.mutex.Unlock()
	return session, nil
}

/* Save (from Store interface) saves off the sessin information to the response.
 * Parameters:
 *     r - The HTTP request object.
 *     w - The response writer object.
 *     session - The session pointer to be saved.
 * Returns:
 *     Standard Go error status.
 */
func (st *AmsterdamStore) Save(r *http.Request, w http.ResponseWriter, session *sessions.Session) error {
	cookie := sessions.NewCookie(session.Name(), session.ID, session.Options)
	http.SetCookie(w, cookie)
	return nil
}

/* sweep sweeps sessions to remove expired ones.
 * Parameters:
 *     tick - Channel that "pulses" periodically to run the task.
 *     done - Channel we write to when we're done.
 */
func (st *AmsterdamStore) sweep(tick <-chan time.Time, done chan bool) {
	for range tick {
		if st.sweepRunning.Load() {
			// phase 1 - identify expired sessions
			st.mutex.RLock()
			zap := make([]string, 0, len(st.sessions))
			for k, v := range st.sessions {
				lastTime, ok := AmSessionGet(v, "lasthit")
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
					AmSessionErase(s)
				}
				st.mutex.Unlock()
			}
		} else {
			break
		}
	}
	done <- true
}

// sessioninfo returns information about the sessions in the store.
func (st *AmsterdamStore) sessionInfo() (int, []string, int) {
	anons := 0
	users := make([]string, 0, len(st.sessions))
	st.mutex.RLock()
	for _, s := range st.sessions {
		v, ok := AmSessionGet(s, "user_anon")
		if ok && v.(bool) {
			anons++
		} else {
			name, _ := AmSessionGet(s, "user_name")
			users = append(users, name.(string))
		}
	}
	st.mutex.RUnlock()
	slices.Sort(users)
	return anons, users, st.maxEntries
}

// SessionStore is the Gorilla session store used by Amsterdam.
var SessionStore *AmsterdamStore

// init registers the time.Time value to be gobbed.
func init() {
	gob.Register(time.Time{})
}

// SetupSessionManager sets up the session manager.
func SetupSessionManager() func() {
	// get the time for the session to expire
	d, err := time.ParseDuration(config.GlobalConfig.Site.SessionExpire)
	if err != nil {
		d, err = time.ParseDuration("1h")
		if err != nil {
			panic(err.Error())
		}
	}

	// create session store
	SessionStore = createAmsterdamStore(d)

	// get the clock value to run sweeps
	d, err = time.ParseDuration("1s")
	if err != nil {
		panic(err.Error())
	}

	// set up the sweep runner
	tkr := time.NewTicker(d)
	done := make(chan bool)
	go SessionStore.sweep(tkr.C, done)
	return func() {
		// stop the sweep runner
		SessionStore.sweepRunning.Store(false)
		<-done
		tkr.Stop()
	}
}

// AmSessionUid returns the current user ID of the session.
func AmSessionUid(session *sessions.Session) (int32, bool) {
	rc, ok := AmSessionGet(session, "user_id")
	if ok {
		return rc.(int32), ok
	} else {
		return -1, ok
	}
}

/* AmSetSessionUser sets the user for the session.
 * Parameters:
 *     session - The session to be updated.
 *     user - The user to be associated with the session.
 */
func AmSetSessionUser(session *sessions.Session, user *database.User) {
	AmSessionPut(session, "user_id", user.Uid)
	AmSessionPut(session, "user_name", user.Username)
	AmSessionPut(session, "user_anon", user.IsAnon)
}

// setSessionAnon sets the user for the session to the anonymous user.
func setSessionAnon(ctx context.Context, session *sessions.Session) {
	u, err := database.AmGetAnonUser(ctx)
	if err == nil {
		AmSetSessionUser(session, u)
	} else {
		log.Errorf("unable to set anonymous user: %v", err)
	}
}

var lastHitMutex sync.Mutex

// AmSessionFirstTime initializes the session after it's first created.
func AmSessionFirstTime(ctx context.Context, session *sessions.Session) {
	lastHitMutex.Lock()
	setSessionAnon(ctx, session)
	AmSessionPut(session, "lasthit", time.Now())
	lastHitMutex.Unlock()
}

// AmResetSession clears the specified session.
func AmResetSession(ctx context.Context, session *sessions.Session) {
	lastHitMutex.Lock()
	AmSessionErase(session)
	setSessionAnon(ctx, session)
	AmSessionPut(session, "lasthit", time.Now())
	lastHitMutex.Unlock()
}

// AmHitSession "hits" a session, updating its "last hit" time.
func AmHitSession(session *sessions.Session) {
	lastHitMutex.Lock()
	AmSessionPut(session, "lasthit", time.Now())
	lastHitMutex.Unlock()
}

/* AmSessions returns information about the currently active sessions.
 * Returns:
 *     Number of users active but not logged in
 *     List of user names currently logged in
 *     Maximum number of users ever in session table.
 */
func AmSessions() (int, []string, int) {
	return SessionStore.sessionInfo()
}

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
	"encoding/gob"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"git.erbosoft.com/amy/amsterdam/config"
	"git.erbosoft.com/amy/amsterdam/database"
	"github.com/google/uuid"
	"github.com/gorilla/sessions"
	"github.com/quasoft/memstore"
	log "github.com/sirupsen/logrus"
)

// SessionStore is the Gorilla session store used by Amsterdam.
var SessionStore sessions.Store

// sessionTable is the global map of all sessions.
var sessionTable map[string]*sessions.Session

// sessionTableMax is the maximum number of entries in the session table.
var sessionTableMax int = 0

// sessionTableMutex is the mutex for the session table.
var sessionTableMutex sync.RWMutex

// sessionExpiry is the amount of time before a session expires.
var sessionExpiry time.Duration

// sweepRunning is the running flag for session sweeping.
var sweepRunning atomic.Bool

// sweepSessions sweeps through the sessions table and removes any expired sessions.
func sweepSessions(tick <-chan time.Time, done chan bool) {
	for range tick {
		if sweepRunning.Load() {
			// phase 1 - identify expired sessions
			sessionTableMutex.RLock()
			zap := make([]string, 0, len(sessionTable))
			for k, v := range sessionTable {
				lastTime := v.Values["lasthit"].(time.Time)
				if time.Since(lastTime) > sessionExpiry {
					zap = append(zap, k)
				}
			}
			sessionTableMutex.RUnlock()

			// phase 2 - get rid of the expired sessions
			for _, k := range zap {
				sessionTableMutex.Lock()
				s := sessionTable[k]
				delete(sessionTable, k)
				sessionTableMutex.Unlock()
				for q := range s.Values {
					delete(s.Values, q)
				}
			}
		} else {
			break
		}
	}
	done <- true
}

// init registers the time.Time value to be gobbed.
func init() {
	gob.Register(time.Time{})
}

// SetupSessionManager sets up the session manager.
func SetupSessionManager() func() {
	// create session store
	SessionStore = memstore.NewMemStore([]byte(config.GlobalConfig.Rendering.CookieKey))

	// create session table
	sessionTable = make(map[string]*sessions.Session)

	// get the time for the session to expire
	d, err := time.ParseDuration(config.GlobalConfig.Site.SessionExpire)
	if err != nil {
		d, err = time.ParseDuration("1h")
		if err != nil {
			panic(err.Error())
		}
	}
	sessionExpiry = d

	// get the clock value to run sweeps
	d, err = time.ParseDuration("1s")
	if err != nil {
		panic(err.Error())
	}

	// set up the sweep runner
	sweepRunning.Store(true)
	tkr := time.NewTicker(d)
	done := make(chan bool)
	go sweepSessions(tkr.C, done)
	return func() {
		// stop the sweep runner
		sweepRunning.Store(false)
		<-done
		tkr.Stop()
	}
}

// AmSessionUid returns the current user ID of the session.
func AmSessionUid(session *sessions.Session) int32 {
	return session.Values["user_id"].(int32)
}

/* AmSetSessionUser sets the user for the session.
 * Parameters:
 *     session - The session to be updated.
 *     user - The user to be associated with the session.
 */
func AmSetSessionUser(session *sessions.Session, user *database.User) {
	session.Values["user_id"] = user.Uid
	session.Values["user_name"] = user.Username
	session.Values["user_anon"] = user.IsAnon
}

// setSessionAnon sets the user for the session to the anonymous user.
func setSessionAnon(session *sessions.Session) {
	u, err := database.AmGetAnonUser()
	if err == nil {
		AmSetSessionUser(session, u)
	} else {
		log.Errorf("unable to set anonymous user: %v", err)
	}
}

// AmSessionFirstTime initializes the session after it's first created.
func AmSessionFirstTime(session *sessions.Session) {
	key := uuid.NewString()
	session.Values["key"] = key
	setSessionAnon(session)
	sessionTableMutex.Lock()
	sessionTable[key] = session
	if len(sessionTable) > sessionTableMax {
		sessionTableMax = len(sessionTable)
	}
	session.Values["lasthit"] = time.Now()
	sessionTableMutex.Unlock()
}

// AmResetSession clears the specified session.
func AmResetSession(session *sessions.Session) {
	key := session.Values["key"]
	for k := range session.Values {
		delete(session.Values, k)
	}
	session.Values["key"] = key
	setSessionAnon(session)
	session.Values["lasthit"] = time.Now()
}

// AmHitSession "hits" a session, updating its "last hit" time.
func AmHitSession(session *sessions.Session) {
	session.Values["lasthit"] = time.Now()
}

/* AmSessions returns information about the currently active sessions.
 * Returns:
 *     Number of users active but not logged in
 *     List of user names currently logged in
 *     Maximum number of users ever in session table.
 */
func AmSessions() (int, []string, int) {
	anons := 0
	users := make([]string, 0, len(sessionTable))
	sessionTableMutex.RLock()
	for _, s := range sessionTable {
		if s.Values["user_anon"].(bool) {
			anons++
		} else {
			users = append(users, s.Values["user_name"].(string))
		}
	}
	sessionTableMutex.RUnlock()
	slices.Sort(users)
	return anons, users, sessionTableMax
}

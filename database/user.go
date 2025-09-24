/*
 * Amsterdam Web Communities System
 * Copyright (c) 2025 Erbosoft Metaverse Design Solutions, All Rights Reserved
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */
// The database package contains database management and storage logic.
package database

import (
	"fmt"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru"
)

// User represents a user in the Amsterdam database.
type User struct {
	Mutex        sync.RWMutex
	Uid          int32      `db:"uid"`
	Username     string     `db:"username"`
	Passhash     string     `db:"passhash"`
	Tokenauth    *string    `db:"tokenauth"`
	ContactID    int32      `db:"contactid"`
	IsAnon       bool       `db:"is_anon"`
	VerifyEMail  bool       `db:"verify_email"`
	Lockout      bool       `db:"lockout"`
	AccessTries  int16      `db:"access_tries"`
	EmailConfNum int32      `db:"email_confnum"`
	BaseLevel    uint16     `db:"base_lvl"`
	Created      time.Time  `db:"created"`
	LastAccess   *time.Time `db:"lastaccess"`
	PassReminder string     `db:"passreminder"`
	Description  *string    `db:"description"`
	DOB          *time.Time `db:"dob"`
}

// userCache is the cache for User objects.
var userCache *lru.TwoQueueCache = nil

// getUserMutex is a mutex on AmGetUser.
var getUserMutex sync.Mutex

// anonUid is the UID of the "anonymous" user.
var anonUid int32 = -1

// init initializes the user cache.
func init() {
	var err error
	userCache, err = lru.New2Q(100)
	if err != nil {
		panic(err)
	}
}

/* AmGetUser returns a reference to the specified user.
 * Parameters:
 *     uid - The UID of the user.
 * Returns:
 *     Pointer to User containing user data, or nil
 *     Standard Go error status
 */
func AmGetUser(uid int32) (*User, error) {
	var err error = nil
	getUserMutex.Lock()
	defer getUserMutex.Unlock()
	rc, ok := userCache.Get(uid)
	if !ok {
		var dbdata []User
		err = amdb.Select(&dbdata, "SELECT * from users WHERE uid = ?", uid)
		if err != nil {
			return nil, err
		}
		if len(dbdata) > 1 {
			return nil, fmt.Errorf("AmGetUser(%d): too many responses(%d)", uid, len(dbdata))
		}
		rc = &(dbdata[0])
		userCache.Add(uid, rc)
	}
	return rc.(*User), err
}

// getAnonUserID retrieves the UID of the "anonymous" user from the database.
func getAnonUserID() (int32, error) {
	if anonUid < 0 {
		rows, err := amdb.Query("SELECT uid FROM users WHERE is_anon = 1")
		if err == nil {
			defer rows.Close()
			if rows.Next() {
				err = rows.Scan(&anonUid)
				if err == nil && rows.Next() {
					err = fmt.Errorf("should be only one anonymous user in Amsterdam database")
				}
			} else {
				err = fmt.Errorf("no anonymous user in Amsterdam database")
			}
		}
		if err != nil {
			return -1, err
		}
	}
	return anonUid, nil
}

/* AmIsUserAnon returns true if the specified user ID is the anonymous one.
 * Parameters:
 *     uid = The user ID to test.
 * Returns:
 *     true if the user is anonymous, false if not
 *     Standard Go error status
 */
func AmIsUserAnon(uid int32) (bool, error) {
	auid, err := getAnonUserID()
	return (uid == auid), err
}

/* AmGetAnonUser returns a reference to the anonymous user.
 * Returns:
 *     Pointer to User containing anonymous user data, or nil
 *     Standard Go error status
 */
func AmGetAnonUser() (*User, error) {
	var rc *User = nil
	auid, err := getAnonUserID()
	if err == nil {
		rc, err = AmGetUser(auid)
	}
	return rc, err
}

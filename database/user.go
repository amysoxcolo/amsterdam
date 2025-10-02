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
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru"
	log "github.com/sirupsen/logrus"
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

// ContactInfo returns the contact info structure for the user.
func (u *User) ContactInfo() (*ContactInfo, error) {
	if u.ContactID < 0 {
		return nil, nil
	}
	return AmGetContactInfo(u.ContactID)
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

/* AmGetUserByName returns a reference to the specified user.
 * Parameters:
 *     name - The username of the user.
 * Returns:
 *     Pointer to User containing user data, or nil
 *     Standard Go error status
 */
func AmGetUserByName(name string) (*User, error) {
	var dbdata []User
	err := amdb.Select(&dbdata, "SELECT * FROM users WHERE username = ?", name)
	if err != nil {
		return nil, err
	}
	if len(dbdata) > 1 {
		return nil, fmt.Errorf("AmGetUserByName(\"%s\"): too many responses(%d)", name, len(dbdata))
	}
	getUserMutex.Lock()
	defer getUserMutex.Unlock()
	rc, ok := userCache.Get(dbdata[0].Uid)
	if !ok {
		rc = &(dbdata[0])
		userCache.Add(dbdata[0].Uid, rc)
	}
	return rc.(*User), nil
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

// hashPassword hashes the password value.
func hashPassword(password string) string {
	if len(password) == 0 {
		return ""
	}
	hasher := sha1.New()
	hasher.Write([]byte(password))
	hashBytes := hasher.Sum(nil)
	return hex.EncodeToString(hashBytes)
}

// touchUser updates the last access time for the user.
func touchUser(user *User) {
	user.Mutex.Lock()
	defer user.Mutex.Unlock()
	moment := time.Now()
	_, _ = amdb.Exec("UPDATE user SET lastaccess = ? WHERE uid = ?", moment, user.Uid)
	user.LastAccess = &moment
}

/* AmAuthenticateUser authenticates a user by name and password.
 * Parameters:
 *     name - The user name to try.
 *     password - The password to try.
 *     remote_ip - The remote IP address, for audit records.
 * Returns:
 *     The User pointer if authenticated, or nil if not.
 *     Standard Go error status.
 */
func AmAuthenticateUser(name string, password string, remote_ip string) (*User, error) {
	log.Debugf("AmAuthenicate() authenticating user %s...", name)
	var ar *AuditRecord = nil
	defer func() {
		if ar != nil {
			go ar.Store()
		}
	}()

	user, err := AmGetUserByName(name)
	if err != nil {
		log.Error("...user not found")
		ar = AmNewAudit(AuditLoginFail, 0, remote_ip, fmt.Sprintf("Bad username: %s", name))
		return nil, errors.New("the user account you have specified does not exist; please try again")
	}
	if user.IsAnon {
		log.Error("...user is the Anonymous Honyak, can't explicitly log in")
		ar = AmNewAudit(AuditLoginFail, user.Uid, remote_ip, "Anonymous user")
		return nil, errors.New("this account cannot be explicitly logged into; please try again")
	}
	if user.Lockout {
		log.Error("...user is locked out by the admin")
		ar = AmNewAudit(AuditLoginFail, user.Uid, remote_ip, "Account locked out")
		return nil, errors.New("this account has been administratively locked; please contact the system administrator for assistance")
	}
	h := hashPassword(password)
	if h != user.Passhash {
		log.Warn("...invalid password")
		ar = AmNewAudit(AuditLoginFail, user.Uid, remote_ip, "Bad password")
		return nil, errors.New("the password you have specified is incorrect; please try again")
	}
	log.Debug("...authenticated")
	touchUser(user)
	ar = AmNewAudit(AuditLoginOK, user.Uid, remote_ip)
	return user, nil
}

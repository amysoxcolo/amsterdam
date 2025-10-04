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
	"hash/crc32"
	"strconv"
	"strings"
	"sync"
	"time"

	"git.erbosoft.com/amy/amsterdam/util"
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

/* NewAuthToken generates and returns a new authentication token for the user.
 * Returns:
 *     Authentication token value
 *	   Standard Go error status.
 */
func (u *User) NewAuthToken() (string, error) {
	if u.IsAnon {
		return "", errors.New("cannot generate token for anonymous user")
	}
	u.Mutex.Lock()
	defer u.Mutex.Unlock()
	newToken := util.GenerateRandomAuthString()
	if _, err := amdb.Exec("UPDATE users SET tokenauth = ? WHERE uid = ?", newToken, u.Uid); err != nil {
		return "", err
	}
	u.Tokenauth = &newToken
	checkValue := uint32(u.Uid) ^ crc32.ChecksumIEEE([]byte(newToken))
	return fmt.Sprintf("AQAT:%d|%s|%d|", u.Uid, newToken, checkValue), nil
}

/* ConfirmEMailAddress checks the E-mail confirmation number and sets "verified" status if it's OK.
 * Parameters:
 *     confnum - The entered confirmation number.
 *     remoteIP - The remote IP address for audit messages.
 * Returns:
 *     Standard Go error status.
 */
func (u *User) ConfirmEMailAddress(confnum int32, remoteIP string) error {
	var ar *AuditRecord = nil
	defer func() {
		AmStoreAudit(ar)
	}()

	log.Debugf("ConfirmEMailAddress for UID %d", u.Uid)
	u.Mutex.Lock()
	defer u.Mutex.Unlock()
	if u.VerifyEMail || AmTestPermission("Global.NoEmailVerify", u.BaseLevel) {
		log.Debug("...user has either already confirmed or is exempt")
		return nil
	}
	if confnum != u.EmailConfNum {
		log.Warn("...confirmation number incorrect")
		ar = AmNewAudit(AuditVerifyEmailFail, u.Uid, remoteIP, "Invalid confirmation number")
		return errors.New("confirmation number is incorrect. Please try again")
	}
	_, err := amdb.Exec("UPDATE users SET verify_email = 1, base_lvl = ? WHERE uid = ?",
		AmDefaultRole("Global.AfterVerify").Level(), u.Uid)
	if err == nil {
		u.VerifyEMail = true
		u.BaseLevel = AmDefaultRole("Global.AfterVerify").Level()
		// TODO: auto-join communities if necessary
		ar = AmNewAudit(AuditVerifyEmailOK, u.Uid, remoteIP)
	}
	return err
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
func AmAuthenticateUser(name string, password string, remoteIP string) (*User, error) {
	log.Debugf("AmAuthenicate() authenticating user %s...", name)
	var ar *AuditRecord = nil
	defer func() {
		AmStoreAudit(ar)
	}()

	user, err := AmGetUserByName(name)
	if err != nil {
		log.Error("...user not found")
		ar = AmNewAudit(AuditLoginFail, 0, remoteIP, fmt.Sprintf("Bad username: %s", name))
		return nil, errors.New("the user account you have specified does not exist; please try again")
	}
	if user.IsAnon {
		log.Error("...user is the Anonymous Honyak, can't explicitly log in")
		ar = AmNewAudit(AuditLoginFail, user.Uid, remoteIP, "Anonymous user")
		return nil, errors.New("this account cannot be explicitly logged into; please try again")
	}
	if user.Lockout {
		log.Error("...user is locked out by the admin")
		ar = AmNewAudit(AuditLoginFail, user.Uid, remoteIP, "Account locked out")
		return nil, errors.New("this account has been administratively locked; please contact the system administrator for assistance")
	}
	h := hashPassword(password)
	if h != user.Passhash {
		log.Warn("...invalid password")
		ar = AmNewAudit(AuditLoginFail, user.Uid, remoteIP, "Bad password")
		return nil, errors.New("the password you have specified is incorrect; please try again")
	}
	log.Debug("...authenticated")
	touchUser(user)
	ar = AmNewAudit(AuditLoginOK, user.Uid, remoteIP)
	return user, nil
}

// crackAuthString validates an auth string and returns its UID and auth token.
func crackAuthString(authString string) (int32, string, error) {
	log.Debug("Decoding authString " + authString)
	if !strings.HasPrefix(authString, "AQAT:") {
		return 0, "", errors.New("prefix not valid")
	}
	parms := strings.Split(authString[5:], "|")
	n1, err := strconv.ParseInt(parms[0], 10, 32)
	if err != nil {
		return 0, "", fmt.Errorf("invalid UID field: %v", err)
	}
	uid := int32(n1)
	n2, err2 := strconv.ParseUint(parms[2], 10, 32)
	if err2 != nil {
		return 0, "", fmt.Errorf("invalid checkvalue field: %v", err2)
	}
	cv1 := uint32(n2)
	cv2 := uint32(uid) ^ crc32.ChecksumIEEE([]byte(parms[1]))
	if cv1 != cv2 {
		return 0, "", errors.New("checkvalues do not match")
	}
	return uid, parms[1], nil
}

/* AmAuthenticateUserByToken authenticates a user via the stored cookie authentication string.
 * Parameters:
 *     authString - The stored cookie authentication string.
 *     remoteIP - The remote IP address wheter trhe user is logging in from.
 * Returns:
 *     Pointer to the authenticated User, or nil.
 *     Standard Go error status.
 */
func AmAuthenticateUserByToken(authString string, remoteIP string) (*User, error) {
	var ar *AuditRecord = nil
	defer func() {
		AmStoreAudit(ar)
	}()

	uid, token, err := crackAuthString(authString)
	if err != nil {
		return nil, fmt.Errorf("authString not valid, ignored: %v", err)
	}
	var user *User
	user, err = AmGetUser(uid)
	if err != nil {
		log.Error("...user not found")
		ar = AmNewAudit(AuditLoginFail, 0, remoteIP, fmt.Sprintf("Bad uid: %d", uid))
		return nil, fmt.Errorf("uid %d not found, ignore: %v", uid, err)
	}
	log.Debugf("AmAuthenicateUserByToken() authenticating user %d...", uid)
	if user.IsAnon {
		log.Error("...user is the Anonymous Honyak, can't explicitly log in")
		ar = AmNewAudit(AuditLoginFail, user.Uid, remoteIP, "Anonymous user")
		return nil, errors.New("this account cannot be explicitly logged into; please try again")
	}
	if user.Lockout {
		log.Error("...user is locked out by the admin")
		ar = AmNewAudit(AuditLoginFail, user.Uid, remoteIP, "Account locked out")
		return nil, errors.New("this account has been administratively locked; please contact the system administrator for assistance")
	}
	if user.Tokenauth == nil || *(user.Tokenauth) != token {
		log.Error("...token mismatch")
		ar = AmNewAudit(AuditLoginFail, user.Uid, remoteIP, "Token mismatch")
		return nil, errors.New("token mismatch")
	}
	log.Debug("...authenticated")
	touchUser(user)
	ar = AmNewAudit(AuditLoginOK, user.Uid, remoteIP)
	return user, nil
}

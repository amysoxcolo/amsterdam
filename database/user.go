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

// UserPrefs represents the user's preferences in a table (one row per user).
type UserPrefs struct {
	Uid        int32  `db:"uid"`
	TimeZoneID string `db:"tzid"`
	LocaleID   string `db:"localeid"`
}

// ReadLocale reads the locale out of the prefs, adjusting for Go use.
func (p *UserPrefs) ReadLocale() string {
	return strings.Replace(p.LocaleID, "_", "-", -1)
}

// WriteLocale writes the locale into the prefs, adjusting for backward compatibility.
func (p *UserPrefs) WriteLocale(loc string) {
	p.LocaleID = strings.Replace(loc, "-", "_", -1)
}

// Clone duplicates the user preferences.
func (p *UserPrefs) Clone() *UserPrefs {
	rc := *p
	return &rc
}

// Save saves off the user preferences, replacing the prefs on the user if necessary.
func (p *UserPrefs) Save(u *User) error {
	if u != nil && u.Uid != p.Uid {
		return errors.New("internal mismatch of IDs")
	}
	_, err := amdb.NamedExec("UPDATE userprefs SET localeid = :localeid, tzid = :tzid WHERE uid = :uid", p)
	if err == nil && u != nil {
		u.prefs = p
	}
	return err
}

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
	flags        *util.OptionSet
	prefs        *UserPrefs
}

// UserProperties represents a property entry for a user.
type UserProperties struct {
	Uid   int32   `db:"uid"`
	Index int32   `db:"ndx"`
	Data  *string `db:"data"`
}

// User property indexes defined.
const (
	UserPropFlags = int32(0) // "flags" user property
)

// Flag values for user property index UserPropFlags defined.
const (
	UserFlagPicturesInPosts  = uint(0)
	UserFlagDisallowSetPhoto = uint(1)
	UserFlagMassMailOptOut   = uint(2)
)

// userCache is the cache for User objects.
var userCache *lru.TwoQueueCache = nil

// getUserMutex is a mutex on AmGetUser.
var getUserMutex sync.Mutex

// userPropCache is the cache for UserProperties objects.
var userPropCache *lru.Cache = nil

// getUserPropMutex is a mutex on AmGetUserProperty.
var getUserPropMutex sync.Mutex

// anonUid is the UID of the "anonymous" user.
var anonUid int32 = -1

// init initializes the caches.
func init() {
	var err error
	userCache, err = lru.New2Q(100)
	if err != nil {
		panic(err)
	}
	userPropCache, err = lru.New(100)
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

// SetContactID sets the contact ID of a user.
func (u *User) SetContactID(cid int32) error {
	u.Mutex.Lock()
	defer u.Mutex.Unlock()
	if _, err := amdb.Exec("UPDATE users SET contactid = ? WHERE uid = ?", cid, u.Uid); err != nil {
		return err
	}
	u.ContactID = cid
	return nil
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
		err = AmAutoJoinCommunities(u)
		if err == nil {
			ar = AmNewAudit(AuditVerifyEmailOK, u.Uid, remoteIP)
		}
	}
	return err
}

// NewEmailConfirmationNumber creates a new confirmation number for a user and saves it off.
func (u *User) NewEmailConfirmationNumber() error {
	u.Mutex.Lock()
	defer u.Mutex.Unlock()
	newnum := util.GenerateRandomConfirmationNumber()
	_, err := amdb.Exec("UPDATE users SET email_confnum = ? WHERE uid = ?", newnum, u.Uid)
	if err != nil {
		u.EmailConfNum = newnum
	}
	return err
}

// ChangePassword resets a user's password.
func (u *User) ChangePassword(password string, remoteIP string) error {
	var ar *AuditRecord = nil
	defer func() {
		AmStoreAudit(ar)
	}()

	u.Mutex.Lock()
	defer u.Mutex.Unlock()
	pval := hashPassword(password)
	_, err := amdb.Exec("UPDATE users SET passhash = ? WHERE uid = ?", pval, u.Uid)
	if err == nil {
		u.Passhash = pval
		ar = AmNewAudit(AuditChangePassword, u.Uid, remoteIP, "via password change request")
	}
	return err
}

// GetFlags retrieves the flags from the properties.
func (u *User) Flags() (*util.OptionSet, error) {
	u.Mutex.Lock()
	defer u.Mutex.Unlock()
	if u.flags == nil {
		s, err := AmGetUserProperty(u.Uid, UserPropFlags)
		if err != nil {
			return nil, err
		}
		if s == nil {
			return nil, fmt.Errorf("missing flags for user %d", u.Uid)
		}
		u.flags = util.OptionSetFromString(*s)
	}
	return u.flags, nil
}

// SaveFlags writes the flags to the database and stores them.
func (u *User) SaveFlags(f *util.OptionSet) error {
	s := f.AsString()
	u.Mutex.Lock()
	defer u.Mutex.Unlock()
	err := AmSetUserProperty(u.Uid, UserPropFlags, &s)
	if err == nil {
		u.flags = f
	}
	return err
}

// FlagValue returns the boolean value of one of the user flags.
func (u *User) FlagValue(ndx uint) bool {
	f, err := u.Flags()
	if err != nil {
		log.Errorf("flag retrieval error for user %d: %v", u.Uid, err)
		return false
	}
	return f.Get(ndx)
}

// Prefs returns the user's preferences record.
func (u *User) Prefs() (*UserPrefs, error) {
	u.Mutex.Lock()
	defer u.Mutex.Unlock()
	if u.prefs == nil {
		var dbdata []UserPrefs
		err := amdb.Select(&dbdata, "SELECT * FROM userprefs WHERE uid = ?", u.Uid)
		if err != nil {
			return nil, err
		}
		if len(dbdata) != 1 {
			return nil, fmt.Errorf("invalid preferences records for user %d", u.Uid)
		}
		u.prefs = &(dbdata[0])
	}
	return u.prefs, nil
}

/* SetProfileData sets the "profile" variables for this user.
 * Parameters:
 *     reminder - Password reminder string.
 *     dob - Date of birth field.
 *     descr - Description string.
 * Returns:
 *     Standard Go error status.
 */
func (u *User) SetProfileData(reminder string, dob *time.Time, descr *string) error {
	u.Mutex.Lock()
	defer u.Mutex.Unlock()
	_, err := amdb.Exec("UPDATE users SET passreminder = ?, dob = ?, description = ? WHERE uid = ?", reminder, dob, descr, u.Uid)
	if err == nil {
		u.PassReminder = reminder
		u.DOB = dob
		u.Description = descr
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

/* AmCreateNewUser creates a new user record in the database.
 * Parameters:
 *     username - New user name.
 *     password - New password.
 *     reminder - Password reminder string.
 *     dob - User date of birth.
 *     remoteIP - Remote IP address for audit record.
 * Returns:
 *     Pointer to new user record.
 *     Standard Go error status.
 */
func AmCreateNewUser(username string, password string, reminder string, dob *time.Time, remoteIP string) (*User, error) {
	var ar *AuditRecord = nil
	defer func() {
		AmStoreAudit(ar)
	}()

	unlock := true
	amdb.Exec("LOCK TABLES users WRITE, userprefs WRITE, propuser WRITE, commmember WRITE, sideboxes WRITE, confhotlist WRITE;")
	defer func() {
		if unlock {
			amdb.Exec("UNLOCK TABLES;")
		}
	}()

	// Test if the user name is already taken.
	rs, err := amdb.Query("SELECT uid FROM users WHERE username = ?", username)
	if err != nil {
		return nil, err
	} else if rs.Next() {
		log.Warnf("username \"%s\" already exists", username)
		return nil, errors.New("that user name already exists. Please try again")
	}

	// Insert the user record.
	_, err2 := amdb.Exec(`INSERT INTO users (username, passhash, verify_email, lockout, email_confnum,
		base_lvl, created, lastaccess, passreminder, description, dob) VALUES (?, ?, 0, 0, ?, ?, NOW(), NOW(), ?, '', ?)`,
		username, hashPassword(password), util.GenerateRandomConfirmationNumber(), AmDefaultRole("Global.NewUser").Level(),
		reminder, dob)
	if err2 != nil {
		return nil, err2
	}
	// Read back the user, which also puts it in the cache.
	user, err3 := AmGetUserByName(username)
	if err3 != nil {
		return nil, err3
	}
	log.Debugf("...created new user \"%s\" with UID %d", username, user.Uid)

	// add user preferences
	_, err = amdb.Exec("INSERT INTO userprefs (uid) VALUES (?)", user.Uid)
	if err != nil {
		return nil, err
	}

	// add user properties
	props := make([]UserProperties, 0)
	anon, _ := getAnonUserID()
	err = amdb.Select(&props, "SELECT * FROM propuser WHERE uid = ?", anon)
	if err != nil {
		return nil, err
	}
	for _, p := range props {
		_, err := amdb.Exec("INSERT INTO propuser (uid, ndx, data) VALUES (?, ?, ?)", user.Uid, p.Index, p.Data)
		if err != nil {
			return nil, err
		}
	}

	// add user sideboxes
	err = copySideboxes(user.Uid, anon)
	if err != nil {
		return nil, err
	}

	amdb.Exec("UNLOCK TABLES;")
	unlock = false

	// auto-join communities
	err = AmAutoJoinCommunities(user)
	if err != nil {
		return nil, err
	}

	// TODO: copy conference hotlists

	// operation was a success - add an audit record
	ar = AmNewAudit(AuditAccountCreated, user.Uid, remoteIP)
	return user, nil
}

func internalGetProp(uid int32, ndx int32) (*UserProperties, error) {
	var err error = nil
	key := fmt.Sprintf("%d:%d", uid, ndx)
	getUserPropMutex.Lock()
	defer getUserPropMutex.Unlock()
	rc, ok := userPropCache.Get(key)
	if !ok {
		var dbdata []UserProperties
		err = amdb.Select(&dbdata, "SELECT * from propuser WHERE uid = ? AND ndx = ?", uid, ndx)
		if err != nil {
			return nil, err
		}
		if len(dbdata) == 0 {
			return nil, nil
		}
		if len(dbdata) > 1 {
			return nil, fmt.Errorf("AmGetUserProperty(%d): too many responses(%d)", uid, len(dbdata))
		}
		rc = &(dbdata[0])
		userPropCache.Add(key, rc)
	}
	return rc.(*UserProperties), nil
}

/* AmGetUserProperty retrieves the value of a user property.
 * Parameters:
 *     uid - The UID of the user to get the property for.
 *     ndx - The index of the property to retrieve.
 * Returns:
 *     Value of the property string.
 *     Standard Go error status.
 */
func AmGetUserProperty(uid int32, ndx int32) (*string, error) {
	p, err := internalGetProp(uid, ndx)
	if err != nil {
		return nil, err
	}
	return p.Data, nil
}

/* AmSetUserProperty sets the value of a user property.
 * Parameters:
 *     uid - The UID of the user to set the property for.
 *     ndx - The index of the property to set.
 *     val - The new value of the property.
 * Returns:
 *     Standard Go error status.
 */
func AmSetUserProperty(uid int32, ndx int32, val *string) error {
	p, err := internalGetProp(uid, ndx)
	if err != nil {
		return err
	}
	getUserPropMutex.Lock()
	defer getUserPropMutex.Unlock()
	if p != nil {
		_, err = amdb.Exec("UPDATE propuser SET data = ? WHERE uid = ? AND ndx = ?", val, uid, ndx)
		if err == nil {
			p.Data = val
		}
	} else {
		prop := UserProperties{Uid: uid, Index: ndx, Data: val}
		_, err := amdb.NamedExec("INSERT INTO propuser (uid, ndx, data) VALUES(:uid, :ndx, :data)", prop)
		if err == nil {
			userPropCache.Add(fmt.Sprintf("%d:%d", uid, ndx), prop)
		}
	}
	return err
}

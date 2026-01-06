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
	"context"
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
	"github.com/jmoiron/sqlx"
	"github.com/klauspost/lctime"
	log "github.com/sirupsen/logrus"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// UserPrefs represents the user's preferences in a table (one row per user).
type UserPrefs struct {
	Uid        int32  `db:"uid"`      // user ID
	TimeZoneID string `db:"tzid"`     // ID of default timezone
	LocaleID   string `db:"localeid"` // ID of default locale
}

// ReadLocale reads the locale out of the prefs, adjusting for Go use.
func (p *UserPrefs) ReadLocale() string {
	return strings.ReplaceAll(p.LocaleID, "_", "-")
}

// WriteLocale writes the locale into the prefs, adjusting for backward compatibility.
func (p *UserPrefs) WriteLocale(loc string) {
	p.LocaleID = strings.ReplaceAll(loc, "-", "_")
}

// Clone duplicates the user preferences.
func (p *UserPrefs) Clone() *UserPrefs {
	rc := *p
	return &rc
}

// Save saves off the user preferences, replacing the prefs on the user if necessary.
func (p *UserPrefs) Save(ctx context.Context, u *User) error {
	if u != nil && u.Uid != p.Uid {
		return errors.New("internal mismatch of IDs")
	}
	_, err := amdb.NamedExecContext(ctx, "UPDATE userprefs SET localeid = :localeid, tzid = :tzid WHERE uid = :uid", p)
	if err == nil && u != nil {
		u.prefs = p
	}
	return err
}

// Localizer returns a localizer for this locale.
func (p *UserPrefs) Localizer() lctime.Localizer {
	lc, err := lctime.NewLocalizer(p.LocaleID)
	if err != nil {
		log.Fatalf("BOGUS LANGUAGE TAG %s in user prefs for uid %d", p.LocaleID, p.Uid)
	}
	return lc
}

// LanguageTag returns the user's language tag.
func (p *UserPrefs) LanguageTag() *language.Tag {
	lt, err := language.Parse(p.ReadLocale())
	if err != nil {
		log.Fatalf("BOGUS LANGUAGE TAG %s in user prefs for uid %d", p.LocaleID, p.Uid)
		return nil
	}
	return &lt
}

// MessagePrinter returns a message printer for the user's selected locale.
func (p *UserPrefs) MessagePrinter() *message.Printer {
	return message.NewPrinter(*p.LanguageTag())
}

// Location returns the time.Location for these user prefs.
func (p *UserPrefs) Location() *time.Location {
	rc, err := time.LoadLocation(p.TimeZoneID)
	if err != nil {
		log.Fatalf("BOGUS TIMEZONE TAG %s in user prefs for uid %d", p.TimeZoneID, p.Uid)
		return time.Local
	}
	return rc
}

// User represents a user in the Amsterdam database.
type User struct {
	Mutex        sync.RWMutex
	Uid          int32      `db:"uid"`           // unique ID of user
	Username     string     `db:"username"`      // user name
	Passhash     string     `db:"passhash"`      // password hash
	Tokenauth    *string    `db:"tokenauth"`     // token authorization information
	ContactID    int32      `db:"contactid"`     // contact information ID
	IsAnon       bool       `db:"is_anon"`       // is this the anonymous user?
	VerifyEMail  bool       `db:"verify_email"`  // is E-mail address verified?
	Lockout      bool       `db:"lockout"`       // is this user locked out?
	AccessTries  int16      `db:"access_tries"`  // how many timews has the user tried to access?
	EmailConfNum int32      `db:"email_confnum"` // E-mail confirmation number
	BaseLevel    uint16     `db:"base_lvl"`      // base access level of the user
	Created      time.Time  `db:"created"`       // account creation time
	LastAccess   *time.Time `db:"lastaccess"`    // last access (login) time
	PassReminder string     `db:"passreminder"`  // last update time
	Description  *string    `db:"description"`   // description
	DOB          *time.Time `db:"dob"`           // date of birth
	flags        *util.OptionSet
	prefs        *UserPrefs
}

// UserProperties represents a property entry for a user.
type UserProperties struct {
	Uid   int32   `db:"uid"`  // UID of user
	Index int32   `db:"ndx"`  // index of property
	Data  *string `db:"data"` // property data
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

// Selectors for field and operator in user search.
const (
	SearchUserFieldName        = 0
	SearchUserFieldDescription = 1
	SearchUserFieldFirstName   = 2
	SearchUserFieldLastName    = 3

	SearchUserOperPrefix    = 0
	SearchUserOperSubstring = 1
	SearchUserOperRegex     = 2
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
func (u *User) ContactInfo(ctx context.Context) (*ContactInfo, error) {
	if u.ContactID < 0 {
		return nil, nil
	}
	return AmGetContactInfo(ctx, u.ContactID)
}

// ContactInfo returns the contact info structure for the user, quietly.
func (u *User) ContactInfoQ(ctx context.Context) *ContactInfo {
	if u.ContactID < 0 {
		return nil
	}
	ci, _ := AmGetContactInfo(ctx, u.ContactID)
	return ci
}

// SetContactID sets the contact ID of a user.
func (u *User) SetContactID(ctx context.Context, cid int32) error {
	u.Mutex.Lock()
	defer u.Mutex.Unlock()
	if _, err := amdb.ExecContext(ctx, "UPDATE users SET contactid = ? WHERE uid = ?", cid, u.Uid); err != nil {
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
func (u *User) NewAuthToken(ctx context.Context) (string, error) {
	if u.IsAnon {
		return "", errors.New("cannot generate token for anonymous user")
	}
	u.Mutex.Lock()
	defer u.Mutex.Unlock()
	newToken := util.GenerateRandomAuthString()
	if _, err := amdb.ExecContext(ctx, "UPDATE users SET tokenauth = ? WHERE uid = ?", newToken, u.Uid); err != nil {
		return "", err
	}
	u.Tokenauth = &newToken
	checkValue := uint32(u.Uid) ^ crc32.ChecksumIEEE([]byte(newToken))
	return fmt.Sprintf("AQAT:%d|%s|%d|", u.Uid, newToken, checkValue), nil
}

/* ConfirmEMailAddress checks the E-mail confirmation number and sets "verified" status if it's OK.
 * Parameters:
 *     ctx - Standard Go context value.
 *     confnum - The entered confirmation number.
 *     remoteIP - The remote IP address for audit messages.
 * Returns:
 *     Standard Go error status.
 */
func (u *User) ConfirmEMailAddress(ctx context.Context, confnum int32, remoteIP string) error {
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
	_, err := amdb.ExecContext(ctx, "UPDATE users SET verify_email = 1, base_lvl = ? WHERE uid = ?",
		AmDefaultRole("Global.AfterVerify").Level(), u.Uid)
	if err == nil {
		u.VerifyEMail = true
		u.BaseLevel = AmDefaultRole("Global.AfterVerify").Level()
		if err = AmAutoJoinCommunities(ctx, u); err == nil {
			ar = AmNewAudit(AuditVerifyEmailOK, u.Uid, remoteIP)
		}
	}
	return err
}

// NewEmailConfirmationNumber creates a new confirmation number for a user and saves it off.
func (u *User) NewEmailConfirmationNumber(ctx context.Context) error {
	u.Mutex.Lock()
	defer u.Mutex.Unlock()
	newnum := util.GenerateRandomConfirmationNumber()
	_, err := amdb.ExecContext(ctx, "UPDATE users SET email_confnum = ? WHERE uid = ?", newnum, u.Uid)
	if err != nil {
		u.EmailConfNum = newnum
	}
	return err
}

// ChangePassword resets a user's password.
func (u *User) ChangePassword(ctx context.Context, password string, remoteIP string) error {
	var ar *AuditRecord = nil
	defer func() {
		AmStoreAudit(ar)
	}()

	u.Mutex.Lock()
	defer u.Mutex.Unlock()
	pval := hashPassword(password)
	_, err := amdb.ExecContext(ctx, "UPDATE users SET passhash = ? WHERE uid = ?", pval, u.Uid)
	if err == nil {
		u.Passhash = pval
		ar = AmNewAudit(AuditChangePassword, u.Uid, remoteIP, "via password change request")
	}
	return err
}

// GetFlags retrieves the flags from the properties.
func (u *User) Flags(ctx context.Context) (*util.OptionSet, error) {
	u.Mutex.Lock()
	defer u.Mutex.Unlock()
	if u.flags == nil {
		s, err := AmGetUserProperty(ctx, u.Uid, UserPropFlags)
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
func (u *User) SaveFlags(ctx context.Context, f *util.OptionSet) error {
	s := f.AsString()
	u.Mutex.Lock()
	defer u.Mutex.Unlock()
	err := AmSetUserProperty(ctx, u.Uid, UserPropFlags, &s)
	if err == nil {
		u.flags = f
	}
	return err
}

// FlagValue returns the boolean value of one of the user flags.
func (u *User) FlagValue(ctx context.Context, ndx uint) bool {
	f, err := u.Flags(ctx)
	if err != nil {
		log.Errorf("flag retrieval error for user %d: %v", u.Uid, err)
		return false
	}
	return f.Get(ndx)
}

// Prefs returns the user's preferences record.
func (u *User) Prefs(ctx context.Context) (*UserPrefs, error) {
	u.Mutex.Lock()
	defer u.Mutex.Unlock()
	if u.prefs == nil {
		var dbdata []UserPrefs
		if err := amdb.SelectContext(ctx, &dbdata, "SELECT * FROM userprefs WHERE uid = ?", u.Uid); err != nil {
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
 *     ctx - Standard Go context value.
 *     reminder - Password reminder string.
 *     dob - Date of birth field.
 *     descr - Description string.
 * Returns:
 *     Standard Go error status.
 */
func (u *User) SetProfileData(ctx context.Context, reminder string, dob *time.Time, descr *string) error {
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
 *	   ctx - Standard Go context value.
 *     uid - The UID of the user.
 * Returns:
 *     Pointer to User containing user data, or nil
 *     Standard Go error status
 */
func AmGetUser(ctx context.Context, uid int32) (*User, error) {
	var err error = nil
	getUserMutex.Lock()
	defer getUserMutex.Unlock()
	rc, ok := userCache.Get(uid)
	if !ok {
		var dbdata []User
		if err = amdb.SelectContext(ctx, &dbdata, "SELECT * from users WHERE uid = ?", uid); err != nil {
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

/* AmGetUserTx returns a reference to the specified user inside a transaction.
 * Parameters:
 *     ctxt - Standard Go context value.
 *     tx - The transaction we're in.
 *     uid - The UID of the user.
 * Returns:
 *     Pointer to User containing user data, or nil
 *     Standard Go error status
 */
func AmGetUserTx(ctx context.Context, tx *sqlx.Tx, uid int32) (*User, error) {
	var err error = nil
	getUserMutex.Lock()
	defer getUserMutex.Unlock()
	rc, ok := userCache.Get(uid)
	if !ok {
		var dbdata []User
		if err = tx.SelectContext(ctx, &dbdata, "SELECT * from users WHERE uid = ?", uid); err != nil {
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
 *     ctx - Standard Go context value.
 *     name - The username of the user.
 *     tx - If this is not nil, use this transaction.
 * Returns:
 *     Pointer to User containing user data, or nil
 *     Standard Go error status
 */
func AmGetUserByName(ctx context.Context, name string, tx *sqlx.Tx) (*User, error) {
	var dbdata []User
	var err error
	if tx != nil {
		err = tx.SelectContext(ctx, &dbdata, "SELECT * FROM users WHERE username = ?", name)
	} else {
		err = amdb.SelectContext(ctx, &dbdata, "SELECT * FROM users WHERE username = ?", name)
	}
	if err != nil {
		return nil, err
	}
	if len(dbdata) > 1 {
		return nil, fmt.Errorf("AmGetUserByName(\"%s\"): too many responses(%d)", name, len(dbdata))
	}
	getUserMutex.Lock()
	rc, ok := userCache.Get(dbdata[0].Uid)
	if !ok {
		rc = &(dbdata[0])
		userCache.Add(dbdata[0].Uid, rc)
	}
	getUserMutex.Unlock()
	return rc.(*User), nil
}

// getAnonUserID retrieves the UID of the "anonymous" user from the database.
func getAnonUserID(ctx context.Context) (int32, error) {
	if anonUid < 0 {
		rows, err := amdb.QueryContext(ctx, "SELECT uid FROM users WHERE is_anon = 1")
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
 *     ctx = Standard Go context value.
 *     uid = The user ID to test.
 * Returns:
 *     true if the user is anonymous, false if not
 *     Standard Go error status
 */
func AmIsUserAnon(ctx context.Context, uid int32) (bool, error) {
	auid, err := getAnonUserID(ctx)
	return (uid == auid), err
}

/* AmGetAnonUser returns a reference to the anonymous user.
 * Parameters:
 *     ctx = Standard Go context value.
 * Returns:
 *     Pointer to User containing anonymous user data, or nil
 *     Standard Go error status
 */
func AmGetAnonUser(ctx context.Context) (*User, error) {
	var rc *User = nil
	auid, err := getAnonUserID(ctx)
	if err == nil {
		rc, err = AmGetUser(ctx, auid)
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
func touchUser(ctx context.Context, tx *sqlx.Tx, user *User) {
	user.Mutex.Lock()
	defer user.Mutex.Unlock()
	moment := time.Now().UTC()
	tx.ExecContext(ctx, "UPDATE user SET lastaccess = ? WHERE uid = ?", moment, user.Uid)
	user.LastAccess = &moment
}

/* AmAuthenticateUser authenticates a user by name and password.
 * Parameters:
 *     ctx - Standard Go context parameter.
 *     name - The user name to try.
 *     password - The password to try.
 *     remote_ip - The remote IP address, for audit records.
 * Returns:
 *     The User pointer if authenticated, or nil if not.
 *     Standard Go error status.
 */
func AmAuthenticateUser(ctx context.Context, name string, password string, remoteIP string) (*User, error) {
	log.Debugf("AmAuthenticateUser() authenticating user %s...", name)
	var ar *AuditRecord = nil
	defer func() {
		AmStoreAudit(ar)
	}()
	success := false
	tx := amdb.MustBegin()
	defer func() {
		if !success {
			tx.Rollback()
		}
	}()

	user, err := AmGetUserByName(ctx, name, tx)
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
	touchUser(ctx, tx, user)
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	success = true
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
 *     ctx - Standard Go context value.
 *     authString - The stored cookie authentication string.
 *     remoteIP - The remote IP address wheter trhe user is logging in from.
 * Returns:
 *     Pointer to the authenticated User, or nil.
 *     Standard Go error status.
 */
func AmAuthenticateUserByToken(ctx context.Context, authString string, remoteIP string) (*User, error) {
	var ar *AuditRecord = nil
	defer func() {
		AmStoreAudit(ar)
	}()
	success := false
	tx := amdb.MustBegin()
	defer func() {
		if !success {
			tx.Rollback()
		}
	}()

	uid, token, err := crackAuthString(authString)
	if err != nil {
		return nil, fmt.Errorf("authString not valid, ignored: %v", err)
	}
	var user *User
	user, err = AmGetUserTx(ctx, tx, uid)
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
	touchUser(ctx, tx, user)
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	success = true
	ar = AmNewAudit(AuditLoginOK, user.Uid, remoteIP)
	return user, nil
}

/* AmCreateNewUser creates a new user record in the database.
 * Parameters:
 *     ctx - Standard Go context value.
 *     username - New user name.
 *     password - New password.
 *     reminder - Password reminder string.
 *     dob - User date of birth.
 *     remoteIP - Remote IP address for audit record.
 * Returns:
 *     Pointer to new user record.
 *     Standard Go error status.
 */
func AmCreateNewUser(ctx context.Context, username string, password string, reminder string, dob *time.Time, remoteIP string) (*User, error) {
	var ar *AuditRecord = nil
	defer func() {
		AmStoreAudit(ar)
	}()
	anon, _ := getAnonUserID(ctx)
	success := false
	tx := amdb.MustBegin()
	defer func() {
		if !success {
			tx.Rollback()
		}
	}()
	unlock := true
	tx.ExecContext(ctx, "LOCK TABLES users WRITE, userprefs WRITE, propuser WRITE, commmember WRITE, sideboxes WRITE, confhotlist WRITE;")
	defer func() {
		if unlock {
			tx.ExecContext(ctx, "UNLOCK TABLES;")
		}
	}()

	// Test if the user name is already taken.
	rs, err := tx.QueryContext(ctx, "SELECT uid FROM users WHERE username = ?", username)
	if err != nil {
		return nil, err
	} else if rs.Next() {
		log.Warnf("username \"%s\" already exists", username)
		return nil, errors.New("that user name already exists. Please try again")
	}

	// Insert the user record.
	_, err2 := tx.ExecContext(ctx, `INSERT INTO users (username, passhash, verify_email, lockout, email_confnum,
		base_lvl, created, lastaccess, passreminder, description, dob) VALUES (?, ?, 0, 0, ?, ?, NOW(), NOW(), ?, '', ?)`,
		username, hashPassword(password), util.GenerateRandomConfirmationNumber(), AmDefaultRole("Global.NewUser").Level(),
		reminder, dob)
	if err2 != nil {
		return nil, err2
	}
	// Read back the user, which also puts it in the cache.
	user, err3 := AmGetUserByName(ctx, username, tx)
	if err3 != nil {
		return nil, err3
	}
	log.Debugf("...created new user \"%s\" with UID %d", username, user.Uid)

	// add user preferences
	_, err = tx.ExecContext(ctx, "INSERT INTO userprefs (uid) VALUES (?)", user.Uid)
	if err != nil {
		return nil, err
	}

	// add user properties
	props := make([]UserProperties, 0)
	if err = tx.SelectContext(ctx, &props, "SELECT * FROM propuser WHERE uid = ?", anon); err != nil {
		return nil, err
	}
	for _, p := range props {
		_, err := tx.ExecContext(ctx, "INSERT INTO propuser (uid, ndx, data) VALUES (?, ?, ?)", user.Uid, p.Index, p.Data)
		if err != nil {
			return nil, err
		}
	}

	// add user sideboxes
	if err = copySideboxes(ctx, tx, user.Uid, anon); err != nil {
		return nil, err
	}

	tx.ExecContext(ctx, "UNLOCK TABLES;")
	unlock = false

	if err = tx.Commit(); err != nil {
		return nil, err
	}
	success = true

	// auto-join communities
	if err = AmAutoJoinCommunities(ctx, user); err != nil {
		return nil, err
	}

	// TODO: copy conference hotlists

	// operation was a success - add an audit record
	ar = AmNewAudit(AuditAccountCreated, user.Uid, remoteIP)
	return user, nil
}

// internalGetProp is a helper used by the property functions.
func internalGetProp(ctx context.Context, uid int32, ndx int32) (*UserProperties, error) {
	var err error = nil
	key := fmt.Sprintf("%d:%d", uid, ndx)
	getUserPropMutex.Lock()
	defer getUserPropMutex.Unlock()
	rc, ok := userPropCache.Get(key)
	if !ok {
		var dbdata []UserProperties
		if err = amdb.SelectContext(ctx, &dbdata, "SELECT * from propuser WHERE uid = ? AND ndx = ?", uid, ndx); err != nil {
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
 *     ctx - Standard Go context value.
 *     uid - The UID of the user to get the property for.
 *     ndx - The index of the property to retrieve.
 * Returns:
 *     Value of the property string.
 *     Standard Go error status.
 */
func AmGetUserProperty(ctx context.Context, uid int32, ndx int32) (*string, error) {
	p, err := internalGetProp(ctx, uid, ndx)
	if err != nil {
		return nil, err
	}
	return p.Data, nil
}

/* AmSetUserProperty sets the value of a user property.
 * Parameters:
 *     ctx - Standard Go context value.
 *     uid - The UID of the user to set the property for.
 *     ndx - The index of the property to set.
 *     val - The new value of the property.
 * Returns:
 *     Standard Go error status.
 */
func AmSetUserProperty(ctx context.Context, uid int32, ndx int32, val *string) error {
	p, err := internalGetProp(ctx, uid, ndx)
	if err != nil {
		return err
	}
	getUserPropMutex.Lock()
	defer getUserPropMutex.Unlock()
	if p != nil {
		_, err = amdb.ExecContext(ctx, "UPDATE propuser SET data = ? WHERE uid = ? AND ndx = ?", val, uid, ndx)
		if err == nil {
			p.Data = val
		}
	} else {
		prop := UserProperties{Uid: uid, Index: ndx, Data: val}
		_, err := amdb.NamedExecContext(ctx, "INSERT INTO propuser (uid, ndx, data) VALUES(:uid, :ndx, :data)", prop)
		if err == nil {
			userPropCache.Add(fmt.Sprintf("%d:%d", uid, ndx), prop)
		}
	}
	return err
}

/* AmSearchUsers searches for users matching certain criteria.
 * Parameters:
 *     ctx - Standard Go context value.
 *     field - A value indicating which field to search:
 *         SearchUserFieldName - The user name.
 *         SearchUserFieldDescription - The user description.
 *         SearchUserFieldFirstName - The user's first name.
 *         SearchUserFieldLastName - The user's last name.
 *     oper - The operation to perform on the search field:
 *         SearchUserOperPrefix - The specified field has the string "term" as a prefix.
 *         SearchUserOperSubstring - The specified field contains the string "term".
 *         SearchUserOperRegex - The specified field matches the regular expression in "term".
 *     term - The search term, as specified above.
 *     offset - Number of users to skip at beginning of list.
 *     max - Maximum number of users to return.
 * Returns:
 *     Array of User pointers representing the return elements.
 *     The total number of users matching this query (could be greater than max)
 *	   Standard Go error status.
 */
func AmSearchUsers(ctx context.Context, field int, oper int, term string, offset int, max int) ([]*User, int, error) {
	var queryPortion strings.Builder
	switch field {
	case SearchUserFieldName:
		queryPortion.WriteString("u.username ")
	case SearchUserFieldDescription:
		queryPortion.WriteString("u.description ")
	case SearchUserFieldFirstName:
		queryPortion.WriteString("c.given_name ")
	case SearchUserFieldLastName:
		queryPortion.WriteString("c.family_name ")
	default:
		return nil, -1, errors.New("invalid field selector")
	}
	switch oper {
	case SearchUserOperPrefix:
		queryPortion.WriteString("LIKE '")
		queryPortion.WriteString(util.SqlEscape(term, true))
		queryPortion.WriteString("%'")
	case SearchUserOperSubstring:
		queryPortion.WriteString("LIKE '%")
		queryPortion.WriteString(util.SqlEscape(term, true))
		queryPortion.WriteString("%'")
	case SearchUserOperRegex:
		queryPortion.WriteString("REGEXP '")
		queryPortion.WriteString(util.SqlEscape(term, false))
		queryPortion.WriteString("'")
	default:
		return nil, -1, errors.New("invalid operator selector")
	}
	q := queryPortion.String()
	rs, err := amdb.QueryContext(ctx, "SELECT COUNT(*) FROM users u, contacts c WHERE u.contactid = c.contactid AND u.is_anon = 0 AND "+q)
	if err != nil {
		return nil, -1, err
	}
	if !rs.Next() {
		return nil, -1, errors.New("internal error getting count")
	}
	var total int
	if err = rs.Scan(&total); err != nil {
		return nil, -1, err
	}
	if total == 0 {
		return make([]*User, 0), 0, nil
	}
	if offset > 0 {
		rs, err = amdb.QueryContext(ctx, "SELECT u.uid FROM users u, contacts c WHERE u.contactid = c.contactid AND u.is_anon = 0 AND "+q+
			" ORDER BY u.username LIMIT ? OFFSET ?", max, offset)
	} else {
		rs, err = amdb.QueryContext(ctx, "SELECT u.uid FROM users u, contacts c WHERE u.contactid = c.contactid AND u.is_anon = 0 AND "+q+
			" ORDER BY u.username LIMIT ?", max)
	}
	if err != nil {
		return nil, total, err
	}
	rc := make([]*User, 0, min(max, 10000))
	for rs.Next() {
		var uid int32
		if err = rs.Scan(&uid); err == nil {
			var u *User
			u, err = AmGetUser(ctx, uid)
			if err == nil {
				rc = append(rc, u)
			}
		}
		if err != nil {
			log.Errorf("AmSearchUsers scan error: %v", err)
		}
	}
	return rc, total, nil
}

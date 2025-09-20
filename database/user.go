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
	"encoding/gob"
	"fmt"
	"time"
)

// User represents a user in the Amsterdam database.
type User struct {
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

// init registers data types from this module.
func init() {
	gob.Register(User{})
}

/* AmGetUser returns a reference to the specified user.
 * Parameters:
 *     uid - The UID of the user.
 * Returns:
 *     Pointer to User containing user data, or nil
 *     Standard Go error status
 */
func AmGetUser(uid int32) (*User, error) {
	var rc []User
	err := amdb.Select(&rc, "SELECT * from users WHERE uid = ?", uid)
	if err != nil {
		return nil, err
	}
	if len(rc) > 1 {
		return nil, fmt.Errorf("AmGetUser(%d): too many responses(%d)", uid, len(rc))
	}
	return &(rc[0]), err
}

/* AmGetAmonUser returns a reference to the anonymous user.
 * Returns:
 *     Pointer to User containing anonymous user data, or nil
 *     Standard Go error status
 */
func AmGetAnonUser() (*User, error) {
	var rc []User
	err := amdb.Select(&rc, "SELECT * from users WHERE is_anon = 1")
	if err != nil {
		return nil, err
	}
	if len(rc) > 1 {
		return nil, fmt.Errorf("AmGetAnonUser: too many responses(%d)", len(rc))
	}
	return &(rc[0]), err
}

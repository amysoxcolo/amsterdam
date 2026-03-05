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
	"time"

	"git.erbosoft.com/amy/amsterdam/util"
)

// PasswordChangeRequest represents a temporary password change request.
type PasswordChangeRequest struct {
	Uid            int32
	Username       string
	Email          string
	Authentication int32
	Expires        time.Time
}

// passwordRequests contains a map of password change requests currently managed.
var passwordRequests map[int32]*PasswordChangeRequest = make(map[int32]*PasswordChangeRequest)

/* AmNewPasswordChangeRequest creates a new password change request and enrolls it.
 * Parameters:
 *     uid - The UID of the user.
 *     username - The user name of the user.
 *     email - The E-mail address of the user.
 * Returns:
 *     Pointer to the new PasswordChangeRequest.
 */
func AmNewPasswordChangeRequest(uid int32, username, email string) *PasswordChangeRequest {
	rc := PasswordChangeRequest{Uid: uid, Username: username, Email: email,
		Authentication: util.GenerateRandomConfirmationNumber(), Expires: time.Now().Add(time.Hour)}
	passwordRequests[uid] = &rc
	return &rc
}

/* AmGetPasswordChangeRequest retrieves the password change request for a UID.
 * Parameters:
 *     uid - The UID to retrieve the request for.
 * Returns:
 *     The PasswordChangeRequest pointer, or nil.
 */
func AmGetPasswordChangeRequest(uid int32) *PasswordChangeRequest {
	rc := passwordRequests[uid]
	if rc != nil {
		delete(passwordRequests, uid)
	}
	return rc
}

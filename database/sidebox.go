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

	"github.com/jmoiron/sqlx"
)

type Sidebox struct {
	Uid      int32   `db:"uid"`
	Boxid    int32   `db:"boxid"`
	Sequence int32   `db:"sequence"`
	Param    *string `db:"param"`
}

// copySideboxes copies sideboxes from one user to another.
func copySideboxes(ctx context.Context, tx *sqlx.Tx, toUid int32, fromUid int32) error {
	sbox := make([]Sidebox, 0, 3)
	err := tx.SelectContext(ctx, &sbox, "SELECT * from sideboxes WHERE uid = ?", fromUid)
	if err == nil {
		for _, sb := range sbox {
			_, err := tx.ExecContext(ctx, "INSERT INTO sideboxes (uid, boxid, sequence, param) VALUES (?, ?, ?, ?)", toUid, sb.Boxid, sb.Sequence, sb.Param)
			if err != nil {
				break
			}
		}
	}
	return err
}

/* AmGetSideboxes returns all the configured sideboxes for a user.
 * Parameters:
 *     ctx = Standard Go context value.
 *     uid = The ID of the user to retrieve sideboxes for.
 * Returns:
 *	   Array of Sidebox structures for the user, or nil
 *     Standard Go error status
 */
func AmGetSideboxes(ctx context.Context, uid int32) ([]*Sidebox, error) {
	sboxes := make([]*Sidebox, 0, 3)
	err := amdb.SelectContext(ctx, &sboxes, "SELECT * FROM sideboxes WHERE uid = ? ORDER BY SEQUENCE", uid)
	return sboxes, err
}

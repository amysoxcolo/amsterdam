/*
 * Amsterdam Web Communities System
 * Copyright (c) 2025-2026 Erbosoft Metaverse Design Solutions, All Rights Reserved
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

// Sidebox represents a user sidebox.
type Sidebox struct {
	Uid      int32   `db:"uid"`      // ID of the user
	Boxid    int32   `db:"boxid"`    // ID of the sidebox
	Sequence int32   `db:"sequence"` // sequence number of the sidebox
	Param    *string `db:"param"`    // parameter string
}

const SIDEBOX_SEQUENCE_SPACING = 100

// Known sidebox IDs.
const (
	SideboxIDCommunities = int32(1)
	SideboxIDConferences = int32(2)
	SideboxIDOnlineUsers = int32(3)
)

// maxSidebox is the maximum sidebox index.
const maxSidebox = SideboxIDOnlineUsers

// copySideboxes copies sideboxes from one user to another.
func copySideboxes(ctx context.Context, tx *sqlx.Tx, toUid int32, fromUid int32) error {
	sbox := make([]Sidebox, 0, maxSidebox)
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

// AmReorderSideboxes changes the position of two sideboxes on the user's list.
func AmReorderSideboxes(ctx context.Context, uid int32, seq1, seq2 int32) error {
	tx, commit, rollback := transaction(ctx)
	defer rollback()

	_, err := tx.ExecContext(ctx, "UPDATE sideboxes SET sequence = -1 WHERE uid = ? AND sequence = ?", uid, seq1)
	if err == nil {
		_, err = tx.ExecContext(ctx, "UPDATE sideboxes SET sequence = ? WHERE uid = ? AND sequence = ?", seq1, uid, seq2)
		if err == nil {
			_, err = tx.ExecContext(ctx, "UPDATE sideboxes SET sequence = ? WHERE uid = ? AND sequence = -1", seq2, uid)
		}
	}
	if err != nil {
		return err
	}
	if err = commit(); err != nil {
		return err
	}
	return nil
}

// AmRemoveSidebox removes a sidebox from the user configuration.
func AmRemoveSidebox(ctx context.Context, uid int32, boxid int32) error {
	tx, commit, rollback := transaction(ctx)
	defer rollback()

	// Get the old sequence number.
	row := tx.QueryRowContext(ctx, "SELECT sequence FROM sideboxes WHERE uid = ? AND boxid = ?", uid, boxid)
	var oldseq int32
	err := row.Scan(&oldseq)
	if err != nil {
		return err
	}

	// Delete the sidebox entry.
	_, err = tx.ExecContext(ctx, "DELETE FROM sideboxes WHERE uid = ? AND boxid = ?", uid, boxid)
	if err == nil {
		// Renumber the other sideboxes to close the gap.
		_, err = tx.ExecContext(ctx, "UPDATE sideboxes SET sequence = sequence - ? WHERE uid = ? AND sequence > ?", SIDEBOX_SEQUENCE_SPACING, uid, oldseq)
	}
	if err != nil {
		return err
	}
	if err = commit(); err != nil {
		return err
	}
	return nil
}

// AmAppendSidebox appends a new sidebox to the existing user's configuration.
func AmAppendSidebox(ctx context.Context, uid int32, boxid int32, param *string) error {
	tx, commit, rollback := transaction(ctx)
	defer rollback()

	row := tx.QueryRowContext(ctx, "SELECT MAX(sequence) FROM sideboxes WHERE uid = ?", uid)
	var topseq int32
	err := row.Scan(&topseq)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "INSERT INTO sideboxes (uid, boxid, sequence, param) VALUES (?, ?, ?, ?)",
		uid, boxid, topseq+SIDEBOX_SEQUENCE_SPACING, param)
	if err != nil {
		return err
	}
	if err = commit(); err != nil {
		return err
	}
	return nil
}

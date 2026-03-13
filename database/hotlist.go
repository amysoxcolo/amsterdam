/*
 * Amsterdam Web Communities System
 * Copyright (c) 2025-2026 Erbosoft Metaverse Design Solutions, All Rights Reserved
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * SPDX-License-Identifier: MPL-2.0
 */
// The database package contains database management and storage logic.
package database

import (
	"context"
	"database/sql"
	"errors"
)

// ConferenceHotlist represents a user's conference hotlist.
type ConferenceHotlist struct {
	Uid      int32 `db:"uid"`
	Sequence int16 `db:"sequence"`
	CommId   int32 `db:"commid"`
	ConfId   int32 `db:"confid"`
}

const HOTLIST_SEQUENCE_SPACING = 100

// Community gets the community pointer from the hotlist.
func (h *ConferenceHotlist) Community(ctx context.Context) (*Community, error) {
	return AmGetCommunity(ctx, h.CommId)
}

// Conference gets the conference pointer from the hotlist.
func (h *ConferenceHotlist) Conference(ctx context.Context) (*Conference, error) {
	return AmGetConference(ctx, h.ConfId)
}

// AmGetConferenceHotlist gets the conference hotlist for a user.
func AmGetConferenceHotlist(ctx context.Context, u *User) ([]ConferenceHotlist, error) {
	var rc []ConferenceHotlist
	err := amdb.SelectContext(ctx, &rc, "SELECT * FROM confhotlist WHERE uid = ? ORDER BY sequence", u.Uid)
	return rc, err
}

// AmCopyConferenceHotlist copies the conference hotlist from one user to another.
func AmCopyConferenceHotlist(ctx context.Context, from, to *User) error {
	hotlist, err := AmGetConferenceHotlist(ctx, from)
	if err != nil {
		return err
	}

	tx, commit, rollback := transaction(ctx)
	defer rollback()
	for _, hl := range hotlist {
		if _, err = tx.ExecContext(ctx, "INSERT INTO confhotlist (uid, sequence, commid, confid) VALUES (?, ?, ?, ?)",
			to.Uid, hl.Sequence, hl.CommId, hl.ConfId); err != nil {
			return err
		}
	}
	if err = commit(); err != nil {
		return err
	}
	return nil
}

// AmReorderHotlist exchanges the position of two items on the user's hotlist.
func AmReorderHotlist(ctx context.Context, u *User, seq1, seq2 int16) error {
	tx, commit, rollback := transaction(ctx)
	defer rollback()

	_, err := tx.ExecContext(ctx, "UPDATE confhotlist SET sequence = -1 WHERE uid = ? AND sequence = ?", u.Uid, seq1)
	if err == nil {
		_, err = tx.ExecContext(ctx, "UPDATE confhotlist SET sequence = ? WHERE uid = ? AND sequence = ?", seq1, u.Uid, seq2)
		if err == nil {
			_, err = tx.ExecContext(ctx, "UPDATE confhotlist SET sequence = ? WHERE uid = ? AND sequence = -1", seq2, u.Uid)
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

// AmRemoveEntryFromHotlist removes an entry from the user's hotlist.
func AmRemoveEntryFromHotlist(ctx context.Context, u *User, seq int16) error {
	tx, commit, rollback := transaction(ctx)
	defer rollback()

	_, err := tx.ExecContext(ctx, "DELETE FROM confhotlist WHERE uid = ? AND sequence = ?", u.Uid, seq)
	if err == nil {
		_, err = tx.ExecContext(ctx, "UPDATE confhotlist SET sequence = sequence - ? WHERE uid = ? AND sequence > ?", HOTLIST_SEQUENCE_SPACING, u.Uid, seq)
	}
	if err != nil {
		return err
	}
	if err = commit(); err != nil {
		return err
	}
	return nil
}

// AmAppendToHotlist adds a community/conference ID to the end of the user's hotlist.
func AmAppendToHotlist(ctx context.Context, u *User, commid, confid int32) error {
	tx, commit, rollback := transaction(ctx)
	defer rollback()

	var newseq int16
	err := tx.GetContext(ctx, &newseq, "SELECT sequence FROM confhotlist WHERE uid = ? AND commid = ? AND confid = ?", u.Uid, commid, confid)
	if err == nil {
		return errors.New("community/conference already exist in hotlist")
	} else if err != sql.ErrNoRows {
		return err
	}
	err = tx.GetContext(ctx, &newseq, "SELECT MAX(sequence) FROM confhotlist WHERE uid = ?", u.Uid)
	if err == sql.ErrNoRows {
		newseq = 0
	} else if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO confhotlist (uid, sequence, commid, confid) VALUES (?, ?, ?, ?)",
		u.Uid, newseq+HOTLIST_SEQUENCE_SPACING, commid, confid)
	if err != nil {
		return err
	}
	if err = commit(); err != nil {
		return err
	}
	return nil
}

// AmIsInHotlist returns true if the community/conference pair is in the hotlist.
func AmIsInHotlist(ctx context.Context, u *User, commid, confid int32) (bool, error) {
	var tmp int16
	err := amdb.GetContext(ctx, &tmp, "SELECT sequence FROM confhotlist WHERE uid = ? AND commid = ? AND confid = ?", u.Uid, commid, confid)
	switch err {
	case nil:
		return true, nil
	case sql.ErrNoRows:
		return false, nil
	}
	return false, err
}

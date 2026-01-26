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

import "context"

// ConferenceHotlist represents a user's conference hotlist.
type ConferenceHotlist struct {
	Uid      int32 `db:"uid"`
	Sequence int16 `db:"sequence"`
	CommId   int32 `db:"commid"`
	ConfId   int32 `db:"confid"`
}

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

	success := false
	tx := amdb.MustBegin()
	defer func() {
		if !success {
			tx.Rollback()
		}
	}()
	for _, hl := range hotlist {
		if _, err = tx.ExecContext(ctx, "INSERT INTO confhotlist (uid, sequence, commid, confid) VALUES (?, ?, ?, ?)",
			to.Uid, hl.Sequence, hl.CommId, hl.ConfId); err != nil {
			return err
		}
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	success = true
	return nil
}

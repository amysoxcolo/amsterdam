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

type Sidebox struct {
	Uid      int32   `db:"uid"`
	Boxid    int32   `db:"boxid"`
	Sequence int32   `db:"sequence"`
	Param    *string `db:"param"`
}

// copySideboxes copies sideboxes from one user to another.
func copySideboxes(toUid int32, fromUid int32) error {
	sbox := make([]Sidebox, 0, 3)
	err := amdb.Select(sbox, "SELECT * from sideboxes WHERE uid = ?", fromUid)
	if err == nil {
		for _, sb := range sbox {
			_, err := amdb.Exec("INSERT INTO sideboxes (uid, boxid, sequence, param) VALUES (?, ?, ?, ?)", toUid, sb.Boxid, sb.Sequence, sb.Param)
			if err != nil {
				break
			}
		}
	}
	return err
}

/* AmGetSideboxes returns all the configured sideboxes for a user.
 * Parameters:
 *     uid = The ID of the user to retrieve sideboxes for.
 * Returns:
 *	   Array of Sidebox structures for the user, or nil
 *     Standard Go error status
 */
func AmGetSideboxes(uid int32) ([]*Sidebox, error) {
	stmt, err := amdb.Preparex("SELECT * FROM sideboxes WHERE uid = ? ORDER BY SEQUENCE")
	if err == nil {
		defer stmt.Close()
		rows, err := stmt.Queryx(uid)
		if err == nil {
			defer rows.Close()
			sboxes := make([]*Sidebox, 0, 3)
			for i := 0; rows.Next(); i++ {
				box := Sidebox{}
				rows.StructScan(&box)
				sboxes = append(sboxes, &box)
			}
			if rows.Err() == nil {
				return sboxes, nil
			}
			return nil, rows.Err()
		}
	}
	return nil, err
}

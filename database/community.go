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
	"fmt"
	"time"

	lru "github.com/hashicorp/golang-lru"
)

// Community struct contains the high level data for a community.
type Community struct {
	Id                int32      `db:"commid"`
	CreateDate        time.Time  `db:"createdate"`
	LastAccess        *time.Time `db:"lastaccess"`
	LastUpdate        *time.Time `db:"lastupdate"`
	ReadLevel         uint16     `db:"read_lvl"`
	WriteLevel        uint16     `db:"write_lvl"`
	CreateLevel       uint16     `db:"create_lvl"`
	JoinLevel         uint16     `db:"join_lvl"`
	ContactId         int32      `db:"contactid"`
	HostUid           *int32     `db:"host_uid"`
	CategoryId        int32      `db:"catid"`
	HideFromDirectory bool       `db:"hide_dir"`
	HideFromSearch    bool       `db:"hide_search"`
	MembersOnly       bool       `db:"membersonly"`
	IsAdmin           bool       `db:"is_admin"`
	InitFeature       int16      `db:"init_ftr"`
	Name              string     `db:"commname"`
	Language          *string    `db:"language"`
	Synopsis          *string    `db:"synopsis"`
	Rules             *string    `dd:"rules"`
	JoinKey           *string    `db:"joinkey"`
	Alias             string     `db:"alias"`
}

// communityCache is the cache for Community objects.
var communityCache *lru.TwoQueueCache = nil

/* AmGetCommunity returns a reference to the specified community.
 * Parameters:
 *     id - The ID of the community.
 * Returns:
 *     Pointer to Community containing community data, or nil
 *     Standard Go error status
 */
func AmGetCommunity(id int32) (*Community, error) {
	var err error = nil
	if communityCache == nil {
		communityCache, err = lru.New2Q(50)
		if err != nil {
			return nil, err
		}
	}
	rc, ok := communityCache.Get(id)
	if !ok {
		var dbdata []Community
		err = amdb.Select(&dbdata, "SELECT * from communities WHERE commid = ?", id)
		if err != nil {
			return nil, err
		}
		if len(dbdata) > 1 {
			return nil, fmt.Errorf("AmGetCommunity(%d): too many responses(%d)", id, len(dbdata))
		}
		rc = &(dbdata[0])
		communityCache.Add(id, rc)
	}
	return rc.(*Community), err
}

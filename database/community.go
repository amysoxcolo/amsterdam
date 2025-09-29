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
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru"
)

// Community struct contains the high level data for a community.
type Community struct {
	Mutex             sync.RWMutex
	Id                int32      `db:"commid"`
	CreateDate        time.Time  `db:"createdate"`
	LastAccess        *time.Time `db:"lastaccess"`
	LastUpdate        *time.Time `db:"lastupdate"`
	ReadLevel         uint16     `db:"read_lvl"`
	WriteLevel        uint16     `db:"write_lvl"`
	CreateLevel       uint16     `db:"create_lvl"`
	DeleteLevel       uint16     `db:"delete_lvl"`
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

// getCommunityMutex is a mutex on AmGetCommunity.
var getCommunityMutex sync.Mutex

// init initializes the community cache.
func init() {
	var err error
	communityCache, err = lru.New2Q(50)
	if err != nil {
		panic(err)
	}
}

/* AmGetCommunity returns a reference to the specified community.
 * Parameters:
 *     id - The ID of the community.
 * Returns:
 *     Pointer to Community containing community data, or nil
 *     Standard Go error status
 */
func AmGetCommunity(id int32) (*Community, error) {
	var err error = nil
	getCommunityMutex.Lock()
	defer getCommunityMutex.Unlock()
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

/* AmGetCommunitiesForUser returns a list of communities the user is a member of.
 * Parameters:
 *     uid - The ID of the user.
 * Returns:
 *	   Array of pointers to communities for the user
 *     Standard Go error status
 */
func AmGetCommunitiesForUser(uid int32) ([]*Community, error) {
	var rc []*Community = make([]*Community, 0)
	rows, err := amdb.Queryx("SELECT commid FROM commmember WHERE uid = ?", uid)
	if err == nil {
		defer rows.Close()
		for err == nil && rows.Next() {
			var cid int32
			var c *Community
			rows.Scan(&cid)
			c, err = AmGetCommunity(cid)
			if err == nil {
				rc = append(rc, c)
			}
		}
	}
	return rc, err
}

/* AmGetCommunityAccessLevel returns the access level of the specified user with respect to the community.
 * This may reflect the user's admin status as well as their status within the community.
 * Parameters:
 *     uid - The UID of the user.
 *     commid - The ID of the community.
 * Returns:
 *     Access level within the community, or 0 if the user is not a member.
 *     Standard Go error status.
 */
func AmGetCommunityAccessLevel(uid int32, commid int32) (uint16, error) {
	var rc uint16 = 0
	rows, err := amdb.Queryx(`SELECT GREATEST(m.granted_lvl, u.base_lvl) AS level FROM users u, commmember m
	    WHERE u.uid = m.uid AND m.uid = ? AND m.commid = ?`, uid, commid)
	if err == nil {
		defer rows.Close()
		if rows.Next() {
			rows.Scan(&rc)
		}
	}
	return rc, err
}

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
	"slices"
	"strconv"
	"sync"
	"time"

	"git.erbosoft.com/amy/amsterdam/util"
	lru "github.com/hashicorp/golang-lru"
	"golang.org/x/text/language"
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

// memberCacheData caches membership information for communities.
type memberCacheData struct {
	isMember bool
	locked   bool
	level    uint16
}

// memberCache contains the memberCacheData entries.
var memberCache *lru.Cache = nil

// memberMutex syncs access to the memberCache.
var memberMutex sync.Mutex

// stuffMembership stuffs a membership record into the cache.
func stuffMembership(cid int32, uid int32, member bool, locked bool, level uint16) {
	key := fmt.Sprintf("%d:%d", cid, uid)
	memberMutex.Lock()
	memberCache.Add(key, &memberCacheData{isMember: member, locked: locked, level: level})
	memberMutex.Unlock()
}

// init initializes the community cache.
func init() {
	var err error
	communityCache, err = lru.New2Q(50)
	if err != nil {
		panic(err)
	}
	memberCache, err = lru.New(250)
	if err != nil {
		panic(err)
	}
}

// Public returns true if the community is public.
func (c *Community) Public() bool {
	return c.JoinKey == nil || *c.JoinKey == ""
}

// ContactInfo returns the contact info structure for the community.
func (c *Community) ContactInfo() (*ContactInfo, error) {
	if c.ContactId < 0 {
		return nil, nil
	}
	return AmGetContactInfo(c.ContactId)
}

// Host returns the reference to the host of the community.
func (c *Community) Host() (*User, error) {
	if c.HostUid == nil {
		return nil, nil
	}
	return AmGetUser(*c.HostUid)
}

func (c *Community) LanguageTag() (*language.Tag, error) {
	if c.Language == nil {
		return nil, nil
	}
	t, err := language.Parse(*c.Language)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

/* Membership returns the details of the specified user's membership in the community.
 * Parameters:
 *     u - The user to check the membership of.
 * Returns:
 *     true if the user is a member, false if not.
 *     true if the user's membership is "locked" (cannot unjoin), false if not.
 *	   User's access level in the community, or 0 if the user is not a member.
 *     Standard Go error status.
 */
func (c *Community) Membership(u *User) (bool, bool, uint16, error) {
	key := fmt.Sprintf("%d:%d", c.Id, u.Uid)
	memberMutex.Lock()
	defer memberMutex.Unlock()
	mbr, ok := memberCache.Get(key)
	if ok {
		m := mbr.(*memberCacheData)
		return m.isMember, m.locked, m.level, nil
	}
	if AmTestPermission("Community.NoJoinRequired", u.BaseLevel) {
		// "no join required" - they are effectively a member, but don't cache that
		return true, false, u.BaseLevel, nil
	}
	rs, err := amdb.Query("SELECT locked, granted_lvl FROM commmember WHERE commid = ? AND uid = ?", c.Id, u.Uid)
	if err == nil {
		if rs.Next() {
			var locked bool
			var level uint16
			rs.Scan(&locked, &level)
			memberCache.Add(key, &memberCacheData{isMember: true, locked: locked, level: level})
			return true, locked, level, nil
		}
		memberCache.Add(key, &memberCacheData{isMember: false, locked: false, level: uint16(0)})
	}
	return false, false, uint16(0), err
}

/* TestPermission is shorthand that tests if a user has a permission with respect to the community.
 * Parameters:
 *     user - The user to be checked.
 *     perm - The permission to be tested.
 * Returns:
 *     true if the user has the permission, false if not.
 *     Standard Go error status.
 */
func (c *Community) TestPermission(user *User, perm string) (bool, error) {
	member, _, level, err := c.Membership(user)
	if err != nil {
		return false, err
	}
	effectiveLevel := user.BaseLevel
	if member && level > effectiveLevel {
		effectiveLevel = level
	}
	return AmTestPermission(perm, effectiveLevel), nil
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
		if len(dbdata) == 0 {
			return nil, fmt.Errorf("community with ID %d not found", id)
		} else if len(dbdata) > 1 {
			return nil, fmt.Errorf("AmGetCommunity(%d): too many responses(%d)", id, len(dbdata))
		}
		rc = &(dbdata[0])
		communityCache.Add(id, rc)
	}
	return rc.(*Community), err
}

/* AmGetCommunityFromParam returns a reference to the specified community based on the parameter.
 * If the parameter is numeric, it's interpreted as a community ID. Otherwise, it's interpreted
 * as a community alias.
 * Parameters:
 *     id - The ID of the community.
 * Returns:
 *     Pointer to Community containing community data, or nil
 *     Standard Go error status
 */
func AmGetCommunityFromParam(param string) (*Community, error) {
	if util.IsNumeric(param) {
		v, _ := strconv.Atoi(param)
		c, err := AmGetCommunity(int32(v))
		if err == nil {
			return c, nil
		}
		// else fall through to trying as alias
	}
	rs, err := amdb.Query("SELECT commid FROM communities WHERE alias = ?", param)
	if err == nil {
		if rs.Next() {
			var cid int32
			rs.Scan(&cid)
			return AmGetCommunity(cid)
		} else {
			return nil, fmt.Errorf("community with alias \"%s\" not found", param)
		}
	}
	return nil, err
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
	var ids []int32 = make([]int32, 0)
	err := amdb.Select(&ids, "SELECT commid FROM commmember WHERE uid = ?", uid)
	if err == nil {
		for _, id := range ids {
			c, err := AmGetCommunity(id)
			if err == nil {
				rc = append(rc, c)
			} else {
				break
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

/* AmAutoJoinCommunities joins the specified user to any communities they're not yet a part of.
 * Parameters:
 *     user - The user to be auto-joined to communities.
 * Returns:
 *     Standard Go error status.
 */
func AmAutoJoinCommunities(user *User) error {
	// get list of current communities
	var current []int32 = make([]int32, 0)
	err := amdb.Select(&current, "SELECT commid FROM commmember WHERE uid = ?", user.Uid)
	if err != nil {
		return err
	}

	// look for candidate communities
	rows, err := amdb.Queryx(`SELECT m.commid, m.locked FROM users u, communities c, commmember m
		WHERE m.uid = u.uid AND m.commid = c.commid AND u.is_anon = 1 AND c.join_lvl <= ?`, user.BaseLevel)
	if err == nil {
		defer rows.Close()
		grantLevel := AmDefaultRole("Community.NewUser").Level()
		for rows.Next() {
			var cid int32
			var lock bool
			rows.Scan(&cid, &lock)
			if !slices.Contains(current, cid) {
				_, err = amdb.Exec("INSERT INTO commmember (commid, uid, granted_lvl, locked) VALUES (?, ?, ?, ?)",
					cid, user.Uid, grantLevel, lock)
				if err != nil {
					break
				}
				stuffMembership(cid, user.Uid, true, lock, grantLevel)
			}
		}
	}
	return err
}

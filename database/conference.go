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
	"errors"
	"fmt"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru"
)

// Conference struct is the top-level structure for a conference.
type Conference struct {
	Mutex       sync.Mutex
	ConfId      int32      `db:"confid"`
	CreateDate  time.Time  `db:"createdate"`
	LastUpdate  *time.Time `db:"lastupdate"`
	ReadLevel   uint16     `db:"read_lvl"`
	PostLevel   uint16     `db:"post_lvl"`
	CreateLevel uint16     `db:"create_lvl"`
	HideLevel   uint16     `db:"hide_lvl"`
	NukeLevel   uint16     `db:"nuke_lvl"`
	ChangeLevel uint16     `db:"change_lvl"`
	DeleteLevel uint16     `db:"delete_lvl"`
	TopTopic    int16      `db:"top_topic"`
	Name        string     `db:"name"`
	Description *string    `db:"descr"`
	IconUrl     *string    `db:"icon_url"`
	Color       *string    `db:"color"`
}

type ConferenceSettings struct {
	ConfId       int32      `db:"confid"`
	Uid          int32      `db:"uid"`
	DefaultPseud *string    `db:"default_pseud"`
	LastRead     *time.Time `db:"last_read"`
	LastPost     *time.Time `db:"last_post"`
}

// conferenceCache is the cache for Conference objects.
var conferenceCache *lru.TwoQueueCache = nil

// getCommunityMutex is a mutex on AmGetCommunity.
var getConferenceMutex sync.Mutex

// conferenceAliasMap stores alias mappings.
var conferenceAliasMap sync.Map

// init initializes the conference cache.
func init() {
	var err error
	conferenceCache, err = lru.New2Q(100)
	if err != nil {
		panic(err)
	}
}

// Aliases returns the list of aliases for this conference.
func (c *Conference) Aliases() ([]string, error) {
	rs, err := amdb.Query("SELECT alias FROM confalias WHERE confid = ? ORDER BY alias", c.ConfId)
	if err != nil {
		return nil, err
	}
	rc := make([]string, 0, 5)
	for rs.Next() {
		var a string
		rs.Scan(&a)
		rc = append(rc, a)
	}
	return rc, nil
}

// AliasesQ returns the list of aliases for this conference, quietly.
func (c *Conference) AliasesQ() []string {
	rc, _ := c.Aliases()
	return rc
}

// Hosts returns the list of users that host this conference.
func (c *Conference) Hosts() ([]*User, error) {
	rs, err := amdb.Query("SELECT uid FROM confmember WHERE confid = ? AND granted_lvl = ?",
		c.ConfId, AmRole("Conference.Host").Level())
	if err != nil {
		return nil, err
	}
	rc := make([]*User, 0, 5)
	for rs.Next() {
		var uid int32
		rs.Scan(&uid)
		u, err := AmGetUser(uid)
		if err == nil {
			rc = append(rc, u)
		}
	}
	return rc, nil
}

// Hosts returns the list of users that host this conference, quietly.
func (c *Conference) HostsQ() []*User {
	rc, _ := c.Hosts()
	return rc
}

// Membership returns a membership flag and granted level for the user in this conference.
func (c *Conference) Membership(u *User) (bool, uint16, error) {
	rs, err := amdb.Query("SELECT granted_lvl FROM confmember WHERE confid = ? AND uid = ?", c.ConfId, u.Uid)
	if err != nil {
		return false, 0, err
	}
	if rs.Next() {
		var level uint16
		rs.Scan(&level)
		return true, level, nil
	}
	return false, 0, nil
}

/* TestPermission is shorthand that tests if a user has a permission with respect to the conference.
 * Parameters:
 *     user - The user to be checked.
 *     perm - The permission to be tested.
 * Returns:
 *     true if the user has the permission, false if not.
 *     Standard Go error status.
 */
func (c *Conference) TestPermission(perm string, level uint16) bool {
	switch perm {
	case "Conference.Read":
		return level >= c.ReadLevel
	case "Conference.Post":
		return level >= c.PostLevel
	case "Conference.Create":
		return level >= c.CreateLevel
	case "Conference.Hide":
		return level >= c.HideLevel
	case "Conference.Nuke":
		return level >= c.NukeLevel
	case "Conference.Change":
		return level >= c.ChangeLevel
	case "Conference.Delete":
		return level >= c.DeleteLevel
	default:
		return AmTestPermission(perm, level)
	}
}

/* AmGetConference returns a conference given its ID.
 * Parameters:
 *     id - The ID of the conference.
 * Returns:
 *     Pointer to the conference, or nil.
 *     Standard Go error status.
 */
func AmGetConference(id int32) (*Conference, error) {
	var err error = nil
	getConferenceMutex.Lock()
	defer getConferenceMutex.Unlock()
	rc, ok := conferenceCache.Get(id)
	if !ok {
		var dbdata []Conference
		err = amdb.Select(&dbdata, "SELECT * from confs where confid = ?", id)
		if err != nil {
			return nil, err
		}
		if len(dbdata) == 0 {
			return nil, fmt.Errorf("conference with ID %d not found", id)
		} else if len(dbdata) > 1 {
			return nil, fmt.Errorf("AmGetConference(%d): too many responses(%d)", id, len(dbdata))
		}
		rc = &(dbdata[0])
		conferenceCache.Add(id, rc)
	}
	return rc.(*Conference), err
}

/* AmGetConferenceByAlias returns a conference given its alias.
 * Parameters:
 *     alias - The alias to look up.
 * Returns:
 *     Pointer to the conference, or nil.
 *     Standard Go error status.
 */
func AmGetConferenceByAlias(alias string) (*Conference, error) {
	var confid int32
	xconf, ok := conferenceAliasMap.Load(alias)
	if ok {
		confid = xconf.(int32)
	} else {
		rs, err := amdb.Query("SELECT confid FROM confalias WHERE alias = ?", alias)
		if err != nil {
			return nil, err
		}
		if !rs.Next() {
			return nil, fmt.Errorf("alias not found: %s", alias)
		}
		rs.Scan(&confid)
		conferenceAliasMap.Store(alias, confid)
	}
	return AmGetConference(confid)
}

/* AmGetConferenceByAliasInCommunity returns a conference in a community given its alias.
 * Parameters:
 *     cid - The community to look inside.
 *     alias - The alias to look up.
 * Returns:
 *     Pointer to the conference, or nil.
 *     Standard Go error status.
 */
func AmGetConferenceByAliasInCommunity(cid int32, alias string) (*Conference, error) {
	rs, err := amdb.Query(`SELECT c.confid FROM commtoconf c, confalias a WHERE c.confid = a.confid
		AND c.commid = ? AND a.alias = ?`, cid, alias)
	if err != nil {
		return nil, err
	}
	if !rs.Next() {
		return nil, errors.New("conference not found")
	}
	var confid int32
	rs.Scan(&confid)
	return AmGetConference(confid)
}

/* AmGetCommunityConferences returns all conferences for a given community.
 * Parameters:
 *     cid - Community ID to get conferences for.
 *     showHidden - true to show hidden conferences.
 * Returns:
 *     Array containing the COnference pointers, or nil.
 *     Stanbard Go error status.
 */
func AmGetCommunityConferences(cid int32, showHidden bool) ([]*Conference, error) {
	q := ""
	if !showHidden {
		q = " AND x.hide_list = 0"
	}
	rs, err := amdb.Query(`SELECT x.confid FROM commtoconf x, confs c WHERE x.confid = c.confid
		AND x.commid = ?`+q+" ORDER BY x.sequence, c.name", cid)
	if err != nil {
		return nil, err
	}
	rc := make([]*Conference, 0, 6)
	for rs.Next() {
		var confid int32
		rs.Scan(&confid)
		conf, err := AmGetConference(confid)
		if err == nil {
			rc = append(rc, conf)
		}
	}
	return rc, nil
}

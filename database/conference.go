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

// Conference struct is the top-level structure for a conference.
type Conference struct {
	ConfId      int32      `db:"confid"`
	CreateDate  time.Time  `db:"createdate"`
	LastUpdate  *time.Time `db:"lastupdate"`
	ReadLevel   uint16     `db:"read_lvl"`
	PostLevel   uint16     `db:"post_lvl"`
	CreateLevel uint16     `db:"create_lvl"`
	HideLevel   uint16     `db:"hide_lvl"`
	NukeLevel   uint16     `db:"nuke_lvl"`
	ChangeLevel uint16     `db:"change_level"`
	DeleteLevel uint16     `db:"delete_level"`
	TopTopic    int16      `db:"top_topic"`
	Name        string     `db:"name"`
	Description *string    `db:"descr"`
	IconUrl     *string    `db:"icon_url"`
	Color       *string    `db:"color"`
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
	confid, ok := conferenceAliasMap.Load(alias)
	if !ok {
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
	return AmGetConference(int32(confid.(int)))
}

/* AmGetCommunityConferences returns all conferences for a given community.
 * Parameters:
 *     cid - COmmunity ID to get conferences for.
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

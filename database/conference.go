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
	"errors"
	"fmt"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
)

// Conference struct is the top-level structure for a conference.
type Conference struct {
	Mutex       sync.Mutex
	ConfId      int32      `db:"confid"`     // unique conference ID
	CreateDate  time.Time  `db:"createdate"` // date of creation
	LastUpdate  *time.Time `db:"lastupdate"` // date of last update
	ReadLevel   uint16     `db:"read_lvl"`   // level required to read
	PostLevel   uint16     `db:"post_lvl"`   // level required to post
	CreateLevel uint16     `db:"create_lvl"` // level required to create topics
	HideLevel   uint16     `db:"hide_lvl"`   // level required to hide posts
	NukeLevel   uint16     `db:"nuke_lvl"`   // level required to nuke posts
	ChangeLevel uint16     `db:"change_lvl"` // level required to change conference
	DeleteLevel uint16     `db:"delete_lvl"` // level required to delete conference
	TopTopic    int16      `db:"top_topic"`  // highest topic number in use
	Name        string     `db:"name"`       // conference name
	Description *string    `db:"descr"`      // conference description
	IconUrl     *string    `db:"icon_url"`   // conference icon URL
	Color       *string    `db:"color"`      // color for conference
}

type ConferenceSettings struct {
	ConfId       int32      `db:"confid"`        // conference ID
	Uid          int32      `db:"uid"`           // user ID
	DefaultPseud *string    `db:"default_pseud"` // default pseud to use in this conference
	LastRead     *time.Time `db:"last_read"`     // last read time
	LastPost     *time.Time `db:"last_post"`     // last post time
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
func (c *Conference) Aliases(ctx context.Context) ([]string, error) {
	rs, err := amdb.QueryContext(ctx, "SELECT alias FROM confalias WHERE confid = ? ORDER BY alias", c.ConfId)
	if err != nil {
		return nil, err
	}
	rc := make([]string, 0, 5)
	for rs.Next() {
		var a string
		if err = rs.Scan(&a); err == nil {
			rc = append(rc, a)
		} else {
			log.Errorf("Aliases scan error: %v", err)
		}
	}
	return rc, nil
}

// AliasesQ returns the list of aliases for this conference, quietly.
func (c *Conference) AliasesQ(ctx context.Context) []string {
	rc, _ := c.Aliases(ctx)
	return rc
}

// Hosts returns the list of users that host this conference.
func (c *Conference) Hosts(ctx context.Context) ([]*User, error) {
	rs, err := amdb.QueryContext(ctx, "SELECT uid FROM confmember WHERE confid = ? AND granted_lvl = ?",
		c.ConfId, AmRole("Conference.Host").Level())
	if err != nil {
		return nil, err
	}
	rc := make([]*User, 0, 5)
	for rs.Next() {
		var uid int32
		if err = rs.Scan(&uid); err == nil {
			u, err := AmGetUser(ctx, uid)
			if err == nil {
				rc = append(rc, u)
			}
		}
	}
	return rc, nil
}

// Hosts returns the list of users that host this conference, quietly.
func (c *Conference) HostsQ(ctx context.Context) []*User {
	rc, _ := c.Hosts(ctx)
	return rc
}

// Membership returns a membership flag and granted level for the user in this conference.
func (c *Conference) Membership(ctx context.Context, u *User) (bool, uint16, error) {
	rs, err := amdb.QueryContext(ctx, "SELECT granted_lvl FROM confmember WHERE confid = ? AND uid = ?", c.ConfId, u.Uid)
	if err != nil {
		return false, 0, err
	}
	rc := false
	if rs.Next() {
		rc = true
		var level uint16
		if err = rs.Scan(&level); err == nil {
			return rc, level, nil
		}
	}
	return rc, 0, err
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

// Settings returns the settings for a user.
func (c *Conference) Settings(ctx context.Context, u *User) (*ConferenceSettings, error) {
	var dbdata []ConferenceSettings
	if err := amdb.SelectContext(ctx, &dbdata, "SELECT * FROM confsettings WHERE confid = ? AND uid = ?", c.ConfId, u.Uid); err != nil {
		return nil, err
	}
	if len(dbdata) == 0 {
		return nil, nil
	}
	if len(dbdata) > 1 {
		return nil, fmt.Errorf("conference.Settings(c=%d,u=%d): too many results (%d)", c.ConfId, u.Uid, len(dbdata))
	}
	return &(dbdata[0]), nil
}

// DefaultPseud returns the default pseud for a user in the conference.
func (c *Conference) DefaultPseud(ctx context.Context, u *User) (string, error) {
	settings, err := c.Settings(ctx, u)
	if err != nil {
		return "", err
	}
	if settings != nil && settings.DefaultPseud != nil {
		return *settings.DefaultPseud, nil
	}
	ci, err := u.ContactInfo(ctx)
	if err != nil {
		return "", err
	}
	return ci.FullName(false), nil
}

// TouchUpdate updates the "last update" date/time in the conference.
func (c *Conference) TouchUpdate(ctx context.Context, tx *sqlx.Tx, lastUpdate time.Time) error {
	_, err := tx.ExecContext(ctx, "UPDATE confs SET lastupdate = ? WHERE confid = ?", lastUpdate, c.ConfId)
	if err == nil {
		c.LastUpdate = &lastUpdate
	}
	return err
}

// TouchRead updates the "last posted" date/time in the conference for the user.
func (c *Conference) TouchRead(ctx context.Context, tx *sqlx.Tx, u *User) (*ConferenceSettings, error) {
	cs, err := c.Settings(ctx, u)
	if err != nil {
		return nil, err
	}
	if !u.IsAnon { // anon user can't update squat
		if cs == nil {
			ci, cerr := u.ContactInfo(ctx)
			if cerr != nil {
				return nil, cerr
			}
			_, err = tx.ExecContext(ctx, "INSERT INTO confsettings (confid, uid, default_pseud, last_read) VALUES (?, ?, ?, NOW())",
				c.ConfId, u.Uid, ci.FullName(false))
		} else {
			_, err = tx.ExecContext(ctx, "UPDATE confsettings SET last_read = NOW() WHERE confid = ? AND uid = ?", c.ConfId, u.Uid)
		}
		if err == nil {
			cs, err = c.Settings(ctx, u) // reread to get updated or inserted values
		}
		if err != nil {
			return nil, err
		}
	}
	return cs, nil
}

// TouchPost updates the "last posted" date/time in the conference for the user.
func (c *Conference) TouchPost(ctx context.Context, tx *sqlx.Tx, u *User, lastPost time.Time) (*ConferenceSettings, error) {
	cs, err := c.Settings(ctx, u)
	if err != nil {
		return nil, err
	}
	if !u.IsAnon { // anon user can't update squat
		if cs == nil {
			ci, cerr := u.ContactInfo(ctx)
			if cerr != nil {
				return nil, cerr
			}
			defaultPseud := ci.FullName(false)
			cs = &ConferenceSettings{
				ConfId:       c.ConfId,
				Uid:          u.Uid,
				DefaultPseud: &defaultPseud,
				LastRead:     &lastPost,
				LastPost:     &lastPost,
			}
			_, err = tx.ExecContext(ctx, "INSERT INTO confsettings (confid, uid, default_pseud, last_read, last_post) VALUES (?, ?, ?, ?, ?)",
				c.ConfId, u.Uid, defaultPseud, lastPost, lastPost)
		} else {
			_, err = tx.ExecContext(ctx, "UPDATE confsettings SET last_post = ? WHERE confid = ? AND uid = ?", lastPost, c.ConfId, u.Uid)
			cs.LastPost = &lastPost
		}
		if err != nil {
			return nil, err
		}
	}
	return cs, nil
}

/* AmGetConference returns a conference given its ID.
 * Parameters:
 *     ctx - Standard Go context value.
 *     id - The ID of the conference.
 * Returns:
 *     Pointer to the conference, or nil.
 *     Standard Go error status.
 */
func AmGetConference(ctx context.Context, id int32) (*Conference, error) {
	var err error = nil
	getConferenceMutex.Lock()
	defer getConferenceMutex.Unlock()
	rc, ok := conferenceCache.Get(id)
	if !ok {
		var dbdata []Conference
		if err = amdb.SelectContext(ctx, &dbdata, "SELECT * from confs where confid = ?", id); err != nil {
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
 *     ctx - Standard Go context value.
 *     alias - The alias to look up.
 * Returns:
 *     Pointer to the conference, or nil.
 *     Standard Go error status.
 */
func AmGetConferenceByAlias(ctx context.Context, alias string) (*Conference, error) {
	var confid int32
	xconf, ok := conferenceAliasMap.Load(alias)
	if ok {
		confid = xconf.(int32)
	} else {
		rs, err := amdb.QueryContext(ctx, "SELECT confid FROM confalias WHERE alias = ?", alias)
		if err != nil {
			return nil, err
		}
		if !rs.Next() {
			return nil, fmt.Errorf("alias not found: %s", alias)
		}
		if err = rs.Scan(&confid); err != nil {
			return nil, err
		}
		conferenceAliasMap.Store(alias, confid)
	}
	return AmGetConference(ctx, confid)
}

/* AmGetConferenceByAliasInCommunity returns a conference in a community given its alias.
 * Parameters:
 *     ctx - Standard Go context value.
 *     cid - The community to look inside.
 *     alias - The alias to look up.
 * Returns:
 *     Pointer to the conference, or nil.
 *     Standard Go error status.
 */
func AmGetConferenceByAliasInCommunity(ctx context.Context, cid int32, alias string) (*Conference, error) {
	rs, err := amdb.QueryContext(ctx, `SELECT c.confid FROM commtoconf c, confalias a WHERE c.confid = a.confid
		AND c.commid = ? AND a.alias = ?`, cid, alias)
	if err != nil {
		return nil, err
	}
	if !rs.Next() {
		return nil, errors.New("conference not found")
	}
	var confid int32
	if err = rs.Scan(&confid); err != nil {
		return nil, err
	}
	return AmGetConference(ctx, confid)
}

/* AmGetCommunityConferences returns all conferences for a given community.
 * Parameters:
 *     ctx - Standard Go context value.
 *     cid - Community ID to get conferences for.
 *     showHidden - true to show hidden conferences.
 * Returns:
 *     Array containing the COnference pointers, or nil.
 *     Stanbard Go error status.
 */
func AmGetCommunityConferences(ctx context.Context, cid int32, showHidden bool) ([]*Conference, error) {
	q := ""
	if !showHidden {
		q = " AND x.hide_list = 0"
	}
	rs, err := amdb.QueryContext(ctx, `SELECT x.confid FROM commtoconf x, confs c WHERE x.confid = c.confid
		AND x.commid = ?`+q+" ORDER BY x.sequence, c.name", cid)
	if err != nil {
		return nil, err
	}
	rc := make([]*Conference, 0, 6)
	for rs.Next() {
		var confid int32
		if err = rs.Scan(&confid); err == nil {
			conf, err := AmGetConference(ctx, confid)
			if err == nil {
				rc = append(rc, conf)
			} else {
				log.Errorf("AmGetCommunityConferences conference error: %v", err)
			}
		} else {
			log.Errorf("AmGetCommunityConferences scan error: %v", err)
		}
	}
	return rc, nil
}

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
	"database/sql"
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
	newflag      bool
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

// Save saves the conference settings.
func (cs *ConferenceSettings) Save(ctx context.Context) error {
	var err error = nil
	if cs.newflag {
		_, err = amdb.ExecContext(ctx, "INSERT INTO confsettings (confid, uid, default_pseud, last_read, last_post) VALUES (?, ?, ?, ?, ?)",
			cs.ConfId, cs.Uid, cs.DefaultPseud, cs.LastRead, cs.LastPost)
		if err == nil {
			cs.newflag = false
		}
	} else {
		_, err = amdb.ExecContext(ctx, "UPDATE confsettings SET default_pseud = ?, last_read = ?, last_post = ? WHERE confid = ? AND uid = ?",
			cs.DefaultPseud, cs.LastRead, cs.LastPost, cs.ConfId, cs.Uid)
	}
	return err
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

// InCommunity returns true if the specified conference is in the community.
func (c *Conference) InCommunity(ctx context.Context, comm *Community) (bool, error) {
	row := amdb.QueryRowContext(ctx, "SELECT commid FROM commtoconf WHERE commid = ? AND confid = ?", comm.Id, c.ConfId)
	var tmp int32
	err := row.Scan(&tmp)
	switch err {
	case nil:
		return true, nil
	case sql.ErrNoRows:
		return false, nil
	}
	return false, err
}

// ContainedBy returns the communities that contain this conference.
func (c *Conference) ContainedBy(ctx context.Context) ([]*Community, error) {
	rs, err := amdb.QueryContext(ctx, "SELECT commid FROM commtoconf WHERE confid = ?", c.ConfId)
	if err != nil {
		return nil, err
	}
	rc := make([]*Community, 0, 1)
	for rs.Next() {
		var cid int32
		if err = rs.Scan(&cid); err != nil {
			return nil, err
		}
		comm, err := AmGetCommunity(ctx, cid)
		if err == nil {
			rc = append(rc, comm)
		} else {
			return nil, err
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
	row := amdb.QueryRowContext(ctx, "SELECT granted_lvl FROM confmember WHERE confid = ? AND uid = ?", c.ConfId, u.Uid)
	var level uint16
	err := row.Scan(&level)
	switch err {
	case nil:
		return true, level, nil
	case sql.ErrNoRows:
		return false, 0, nil
	}
	return false, 0, err
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
		settings := ConferenceSettings{
			ConfId:       c.ConfId,
			Uid:          u.Uid,
			DefaultPseud: nil,
			LastRead:     nil,
			LastPost:     nil,
			newflag:      true,
		}
		return &settings, nil
	}
	if len(dbdata) > 1 {
		return nil, fmt.Errorf("conference.Settings(c=%d,u=%d): too many results (%d)", c.ConfId, u.Uid, len(dbdata))
	}
	dbdata[0].newflag = false
	return &(dbdata[0]), nil
}

// Link returns a link string to this conference.
func (c *Conference) Link(ctx context.Context, scope string) (string, error) {
	aliases, err := c.Aliases(ctx)
	if err != nil {
		return "", err
	}
	if scope == "community" {
		return fmt.Sprintf("%s.", aliases[0]), nil
	}
	if scope == "global" {
		comms, err := c.ContainedBy(ctx)
		if err == nil {
			return fmt.Sprintf("%s!%s", comms[0].Alias, aliases[0]), nil
		}
		return "", err
	}
	return "", errors.New("invalid scope")
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

// SetDefaultPseud sets the default pseud for a user in the conference.
func (c *Conference) SetDefaultPseud(ctx context.Context, u *User, pseud string) error {
	if u.IsAnon {
		return nil
	}
	settings, err := c.Settings(ctx, u)
	if err != nil {
		return err
	}
	settings.DefaultPseud = &pseud
	return settings.Save(ctx)
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
		if cs.newflag {
			err = cs.Save(ctx)
			if err != nil {
				return cs, err
			}
		}
		_, err = tx.ExecContext(ctx, "UPDATE confsettings SET last_read = NOW() WHERE confid = ? AND uid = ?", c.ConfId, u.Uid)
		if err == nil {
			cs, err = c.Settings(ctx, u) // reread settings
		}
	}
	return cs, err
}

// TouchPost updates the "last posted" date/time in the conference for the user.
func (c *Conference) TouchPost(ctx context.Context, tx *sqlx.Tx, u *User, lastPost time.Time) (*ConferenceSettings, error) {
	cs, err := c.Settings(ctx, u)
	if err != nil {
		return nil, err
	}
	if !u.IsAnon { // anon user can't update squat
		if cs.newflag {
			err = cs.Save(ctx)
			if err != nil {
				return cs, err
			}
		}
		_, err = tx.ExecContext(ctx, "UPDATE confsettings SET last_post = NOW() WHERE confid = ? AND uid = ?", c.ConfId, u.Uid)
		if err == nil {
			cs, err = c.Settings(ctx, u) // reread settings
		}
	}
	return cs, err
}

// UnreadMessages returns the total number of unread messages in a conference for a user.
func (c *Conference) UnreadMessages(ctx context.Context, u *User) (int32, error) {
	row := amdb.QueryRowContext(ctx, `SELECT SUM(t.top_message - IFNULL(s.last_message,-1))
		FROM topics t LEFT JOIN topicsettings s ON t.topicid = s.topicid AND s.uid = ?
		WHERE t.confid = ? AND t.archived = 0 AND (s.hidden IS NULL OR s.hidden = 0)`, u.Uid, c.ConfId)
	var rc int32
	err := row.Scan(&rc)
	return rc, err
}

// fixseenData is a temporary structure used in assisting with Fixseen.
type fixseenData struct {
	topicid    int32
	topmessage int32
	insert     bool
}

// Fixseen marks all messages in a conference as read.
func (c *Conference) Fixseen(ctx context.Context, u *User) error {
	if u.IsAnon {
		return nil
	}
	success := false
	tx := amdb.MustBegin()
	defer func() {
		if !success {
			tx.Rollback()
		}
	}()

	// Get a count of topics beforehand.
	row := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM topics WHERE confid = ?", c.ConfId)
	count := 0
	err := row.Scan(&count)
	if err != nil {
		return err
	}

	// Build the list of all topics.
	rs, err := tx.QueryContext(ctx, `SELECT t.topicid, t.top_message, ISNULL(s.last_message) FROM topics t
		LEFT JOIN topicsettings s ON t.topicid = s.topicid AND s.uid = ? WHERE t.confid = ?`, u.Uid, c.ConfId)
	if err != nil {
		return err
	}
	work := make([]fixseenData, 0, count)
	for rs.Next() {
		var d fixseenData
		err = rs.Scan(&(d.topicid), &(d.topmessage), &(d.insert))
		work = append(work, d)
	}

	// Adjust each topic in turn.
	for _, d := range work {
		if d.insert {
			_, err = tx.ExecContext(ctx, "INSERT INTO topicsettings (topicid, uid, last_message, last_read) VALUES (?, ?, ?, NOW())", d.topicid, u.Uid, d.topmessage)
		} else {
			_, err = tx.ExecContext(ctx, "UPDATE topicsettings SET last_message = ?, last_read = NOW() WHERE topicid = ? AND uid = ?", d.topmessage, d.topicid, u.Uid)
		}
		if err != nil {
			return err
		}
	}

	// Also update last-read in conference.
	if _, err = c.TouchRead(ctx, tx, u); err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return err
	}
	success = true
	return nil
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
		row := amdb.QueryRowContext(ctx, "SELECT confid FROM confalias WHERE alias = ?", alias)
		err := row.Scan(&confid)
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("alias not found: %s", alias)
		} else if err != nil {
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
	row := amdb.QueryRowContext(ctx, `SELECT c.confid FROM commtoconf c, confalias a WHERE c.confid = a.confid
		AND c.commid = ? AND a.alias = ?`, cid, alias)
	var confid int32
	err := row.Scan(&confid)
	switch err {
	case nil:
		return AmGetConference(ctx, confid)
	case sql.ErrNoRows:
		return nil, errors.New("conference not found")
	}
	return nil, err
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

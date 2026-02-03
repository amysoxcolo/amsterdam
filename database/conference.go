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

	"git.erbosoft.com/amy/amsterdam/util"
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
	flags       *util.OptionSet
}

type ConferenceSettings struct {
	ConfId       int32      `db:"confid"`        // conference ID
	Uid          int32      `db:"uid"`           // user ID
	DefaultPseud *string    `db:"default_pseud"` // default pseud to use in this conference
	LastRead     *time.Time `db:"last_read"`     // last read time
	LastPost     *time.Time `db:"last_post"`     // last post time
	newflag      bool
}

// ConferenceProperties represents a property entry for a conference.
type ConferenceProperties struct {
	ConfId int32   `db:"confid"` // conference ID
	Index  int32   `db:"ndx"`    // property index
	Data   *string `db:"data"`   // property data
}

// Default spacing between sequence numbers in commtoconf table.
const COMMTOCONF_SEQ_SPACING = 10

// Conference property indexes defined.
const (
	ConferencePropFlags = int32(0)
)

// Flag values for conference property index ConferencePropFlags defined.
const (
	ConferenceFlagPicturesInPosts = uint(0)
)

// conferenceCache is the cache for Conference objects.
var conferenceCache *lru.TwoQueueCache = nil

// getCommunityMutex is a mutex on AmGetCommunity.
var getConferenceMutex sync.Mutex

// conferenceAliasMap stores alias mappings.
var conferenceAliasMap sync.Map

// conferencePropCache is the cache for ConferenceProperties objects.
var conferencePropCache *lru.Cache = nil

// getConferencePropMutex is a mutex on AmGetConferenceProperty.
var getConferencePropMutex sync.Mutex

// init initializes the conference cache.
func init() {
	var err error
	conferenceCache, err = lru.New2Q(100)
	if err != nil {
		panic(err)
	}
	conferencePropCache, err = lru.New(100)
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

// HiddenInList returns whether or not this conference is hidden in the community's conference list.
func (c *Conference) HiddenInList(ctx context.Context, comm *Community) (bool, error) {
	row := amdb.QueryRowContext(ctx, "SELECT hide_list FROM commtoconf WHERE commid = ? AND confid = ?", comm.Id, c.ConfId)
	var rc bool
	err := row.Scan(&rc)
	switch err {
	case nil:
		return rc, nil
	case sql.ErrNoRows:
		return false, errors.New("conference not in community")
	}
	return false, err
}

// SetHiddenInList sets whether or not this conference is hidden in the community's conference list.
func (c *Conference) SetHiddenInList(ctx context.Context, comm *Community, flag bool) error {
	_, err := amdb.ExecContext(ctx, "UPDATE commtoconf SET hide_list = ? WHERE commid = ? AND confid = ?", flag, comm.Id, c.ConfId)
	return err
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

// Flags retrieves the flags from the properties.
func (c *Conference) Flags(ctx context.Context) (*util.OptionSet, error) {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()
	if c.flags == nil {
		s, err := AmGetConferenceProperty(ctx, c.ConfId, ConferencePropFlags)
		if err != nil {
			return nil, err
		}
		if s == nil {
			c.flags = util.NewOptionSet()
		} else {
			c.flags = util.OptionSetFromString(*s)
		}
	}
	return c.flags, nil
}

// SaveFlags writes the flags to the database and stores them.
func (c *Conference) SaveFlags(ctx context.Context, f *util.OptionSet) error {
	s := f.AsString()
	c.Mutex.Lock()
	defer c.Mutex.Unlock()
	err := AmSetConferenceProperty(ctx, c.ConfId, ConferencePropFlags, &s)
	if err == nil {
		c.flags = f
	}
	return err
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

// SetInfo sets the name, pseud, and security levels on a conference.
func (c *Conference) SetInfo(ctx context.Context, name, descr string, read_lvl, post_lvl, create_lvl, hide_lvl, nuke_lvl, change_lvl, delete_lvl uint16) error {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()
	_, err := amdb.ExecContext(ctx, `UPDATE confs SET name = ?, descr = ?, read_lvl = ?, post_lvl = ?, create_lvl = ?,
		hide_lvl = ?, nuke_lvl = ?, change_lvl = ?, delete_lvl = ?, lastupdate = NOW() WHERE confid = ?`, name, descr, read_lvl, post_lvl,
		create_lvl, hide_lvl, nuke_lvl, change_lvl, delete_lvl, c.ConfId)
	if err == nil {
		var tmp []Conference
		err := amdb.SelectContext(ctx, &tmp, "SELECT * FROM confs WHERE confid = ?", c.ConfId)
		if err == nil {
			if len(tmp) != 1 {
				err = errors.New("internal error rereading conference")
			} else {
				c.Name = tmp[0].Name
				c.Description = tmp[0].Description
				c.ReadLevel = tmp[0].ReadLevel
				c.PostLevel = tmp[0].PostLevel
				c.CreateLevel = tmp[0].CreateLevel
				c.HideLevel = tmp[0].HideLevel
				c.NukeLevel = tmp[0].NukeLevel
				c.ChangeLevel = tmp[0].ChangeLevel
				c.DeleteLevel = tmp[0].DeleteLevel
				c.LastUpdate = tmp[0].LastUpdate
			}
		}
	}
	return err
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

// internalGetConfProp is a helper used by the conference property functions.
func internalGetConfProp(ctx context.Context, confid int32, ndx int32) (*ConferenceProperties, error) {
	var err error = nil
	key := fmt.Sprintf("%d:%d", confid, ndx)
	getConferencePropMutex.Lock()
	defer getConferencePropMutex.Unlock()
	rc, ok := conferencePropCache.Get(key)
	if !ok {
		var dbdata []ConferenceProperties
		if err = amdb.SelectContext(ctx, &dbdata, "SELECT * from propconf WHERE confid = ? AND ndx = ?", confid, ndx); err != nil {
			return nil, err
		}
		if len(dbdata) == 0 {
			return nil, nil
		}
		if len(dbdata) > 1 {
			return nil, fmt.Errorf("AmGetConferenceProperty(%d): too many responses(%d)", confid, len(dbdata))
		}
		rc = &(dbdata[0])
		conferencePropCache.Add(key, rc)
	}
	return rc.(*ConferenceProperties), nil
}

/* AmGetConferenceProperty retrieves the value of a conference property.
 * Parameters:
 *     ctx - Standard Go context value.
 *     confid - The ID of the conference to get the property for.
 *     ndx - The index of the property to retrieve.
 * Returns:
 *     Value of the property string.
 *     Standard Go error status.
 */
func AmGetConferenceProperty(ctx context.Context, confid int32, ndx int32) (*string, error) {
	p, err := internalGetConfProp(ctx, confid, ndx)
	if err != nil {
		return nil, err
	} else if p == nil {
		return nil, nil
	}
	return p.Data, nil
}

/* AmSetConferenceProperty sets the value of a conference property.
 * Parameters:
 *     ctx - Standard Go context value.
 *     confid - The ID of the conference to set the property for.
 *     ndx - The index of the property to set.
 *     val - The new value of the property.
 * Returns:
 *     Standard Go error status.
 */
func AmSetConferenceProperty(ctx context.Context, confid int32, ndx int32, val *string) error {
	p, err := internalGetConfProp(ctx, confid, ndx)
	if err != nil {
		return err
	}
	getConferencePropMutex.Lock()
	defer getConferencePropMutex.Unlock()
	if p != nil {
		_, err = amdb.ExecContext(ctx, "UPDATE propconf SET data = ? WHERE confid = ? AND ndx = ?", val, confid, ndx)
		if err == nil {
			p.Data = val
		}
	} else {
		prop := ConferenceProperties{ConfId: confid, Index: ndx, Data: val}
		_, err := amdb.NamedExecContext(ctx, "INSERT INTO propconf (confid, ndx, data) VALUES(:confid, :ndx, :data)", prop)
		if err == nil {
			conferencePropCache.Add(fmt.Sprintf("%d:%d", confid, ndx), prop)
		}
	}
	return err
}

/* AmCreateConference creates a new conference.
 * Parameters:
 *     ctx - Standard Go context value.
 *     comm - Community to create this conference in.
 *     name - New conference name.
 *     alias - New conference alias.
 *     descr - Nw conference description.
 *     private - true to create a private conference, false to create a public one.
 *     hide_list - true if the conference should be hidden in the community conference list.
 *     u - User creating the conference; this user will become the conference host.
 *     ipaddr - IP address of the creation request.
 * Returns:
 *     Pointer to the new conference, or nil.
 *     Standard Go error status.
 */
func AmCreateConference(ctx context.Context, comm *Community, name, alias, descr string, private, hide_list bool, u *User, ipaddr string) (*Conference, error) {
	newConf := Conference{
		Name:        name,
		HideLevel:   AmRoleList("Conference.Hide").Default().Level(),
		NukeLevel:   AmRoleList("Conference.Nuke").Default().Level(),
		ChangeLevel: AmRoleList("Conference.Change").Default().Level(),
		DeleteLevel: AmRoleList("Conference.Delete").Default().Level(),
	}
	if descr != "" {
		newConf.Description = &descr
	}
	if private {
		newConf.ReadLevel = AmDefaultRole("Conference.Read.Private").Level()
		newConf.PostLevel = AmDefaultRole("Conference.Post.Private").Level()
		newConf.CreateLevel = AmDefaultRole("Conference.Create.Private").Level()
	} else {
		newConf.ReadLevel = AmDefaultRole("Conference.Read.Public").Level()
		newConf.PostLevel = AmDefaultRole("Conference.Post.Public").Level()
		newConf.CreateLevel = AmDefaultRole("Conference.Create.Public").Level()
	}

	var ar *AuditRecord = nil
	defer func() {
		AmStoreAudit(ar)
	}()
	success := false
	tx := amdb.MustBegin()
	defer func() {
		if !success {
			tx.Rollback()
		}
	}()
	getConferenceMutex.Lock()
	defer getConferenceMutex.Unlock()

	// Ensure the alias is not in use.
	row := tx.QueryRowContext(ctx, "SELECT confid FROM confalias WHERE alias = ?", alias)
	var tmp int32
	err := row.Scan(&tmp)
	if err == nil {
		return nil, fmt.Errorf("the alias '%s' is already in use by a different conference", alias)
	} else if err != sql.ErrNoRows {
		return nil, err
	}

	// Create the conference record and then reload it so we have its ID available.
	rs, err := tx.NamedExecContext(ctx, `INSERT INTO confs (createdate, read_lvl, post_lvl, create_lvl, hide_lvl, nuke_lvl, change_lvl, delete_lvl, name, descr)
		VALUES (NOW(), :read_lvl, :post_lvl, :create_lvl, :hide_lvl, :nuke_lvl, :change_lvl, :delete_lvl, :name, :descr)`, &newConf)
	if err != nil {
		return nil, err
	}
	newId, err := rs.LastInsertId()
	if err != nil {
		return nil, err
	}
	var rc []Conference
	err = tx.SelectContext(ctx, &rc, "SELECT * FROM confs WHERE confid = ?", int32(newId))
	if err != nil {
		return nil, err
	} else if len(rc) != 1 {
		return nil, errors.New("internal error reading back conference")
	}

	// Attach the alias to the conference.
	_, err = tx.ExecContext(ctx, "INSERT INTO confalias (confid, alias) VALUES (?, ?)", rc[0].ConfId, alias)
	if err != nil {
		return nil, err
	}

	// Get the current "last" sequence number.
	row = tx.QueryRowContext(ctx, "SELECT MAX(sequence) FROM commtoconf WHERE commid = ?", comm.Id)
	var seq int
	err = row.Scan(&seq)
	if err != nil {
		return nil, err
	}

	// Link the conference into the community, and set the hide flag.
	_, err = tx.ExecContext(ctx, "INSERT INTO commtoconf (commid, confid, sequence, hide_list) VALUES (?, ?, ?, ?)", comm.Id, rc[0].ConfId,
		int16(seq+COMMTOCONF_SEQ_SPACING), hide_list)
	if err != nil {
		return nil, err
	}

	// Make the specified user the first host of the conference.
	_, err = tx.ExecContext(ctx, "INSERT INTO confmember (confid, uid, granted_lvl) VALUES (?, ?, ?)", rc[0].ConfId, u.Uid, AmDefaultRole("Conference.NewHost").Level())
	if err != nil {
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}
	success = true

	// Add the new conference to the cache, and create our audit record.
	conferenceCache.Add(rc[0].ConfId, &(rc[0]))
	ar = AmNewAudit(AuditConferenceCreate, u.Uid, ipaddr, fmt.Sprintf("confid=%d", rc[0].ConfId), fmt.Sprintf("name=%s", name), fmt.Sprintf("alias=%s", alias))
	return &(rc[0]), nil
}

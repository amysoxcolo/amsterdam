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
	"slices"
	"strings"
	"sync"
	"time"

	"git.erbosoft.com/amy/amsterdam/config"
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

// ConferenceSettings represents a user's settings within the conference.
type ConferenceSettings struct {
	ConfId       int32      `db:"confid"`        // conference ID
	Uid          int32      `db:"uid"`           // user ID
	DefaultPseud *string    `db:"default_pseud"` // default pseud to use in this conference
	LastRead     *time.Time `db:"last_read"`     // last read time
	LastPost     *time.Time `db:"last_post"`     // last post time
	newflag      bool
}

// ConferenceMember represents the membership entries in a conference.
type ConferenceMember struct {
	ConfId int32  `db:"confid"`      // conference ID
	Uid    int32  `db:"uid"`         // user ID
	Level  uint16 `db:"granted_lvl"` // level granted within the conference
}

// ConferenceProperties represents a property entry for a conference.
type ConferenceProperties struct {
	ConfId int32   `db:"confid"` // conference ID
	Index  int32   `db:"ndx"`    // property index
	Data   *string `db:"data"`   // property data
}

// ConferenceSummary represents summary information about a conference.
type ConferenceSummary struct {
	ConfId      int32      // conference ID
	Name        string     // conference name
	Alias       string     // an alias for the conference
	LastUpdate  *time.Time // last update date/time
	Hosts       []string   // usernames of the hosts
	Description string     // description string
	Sequence    int16      // sequence number in the list
	Hidden      bool       // hidden in list?
}

// Conf gets the conference from the summary.
func (cs *ConferenceSummary) Conf(ctx context.Context) (*Conference, error) {
	return AmGetConference(ctx, cs.ConfId)
}

// Default spacing between sequence numbers in commtoconf table.
const COMMTOCONF_SEQ_SPACING = 10

// Conference property indexes defined.
const (
	ConferencePropFlags = int32(0)
)

// Flag values for conference property index ConferencePropFlags defined.
const (
	ConferenceFlagPicturesInPosts  = uint(0) // show pictures in posts
	ConferenceFlagBuggyAttachments = uint(1) // buggy attachment behavior
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

// setupConferenceCache initializes the conference cache.
func setupConferenceCache() {
	var err error
	conferenceCache, err = lru.New2Q(config.GlobalConfig.Tuning.Caches.Conferences)
	if err != nil {
		panic(err)
	}
	conferencePropCache, err = lru.New(config.GlobalConfig.Tuning.Caches.ConferenceProps)
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

// AddAlias adds an alias to the conference.
func (c *Conference) AddAlias(ctx context.Context, alias string, u *User, comm *Community, ipaddr string) error {
	row := amdb.QueryRowContext(ctx, "SELECT alias FROM confalias WHERE confid = ? AND alias = ?", c.ConfId, alias)
	tmp := ""
	err := row.Scan(&tmp)
	if err != sql.ErrNoRows {
		if err == nil {
			return fmt.Errorf("the alias '%s' is already in use by another conference", alias)
		}
		return err
	}
	_, err = amdb.ExecContext(ctx, "INSERT INTO confalias (confid, alias) VALUES (?, ?)", c.ConfId, alias)
	if err != nil {
		return err
	}

	AmStoreAudit(AmNewCommAudit(AuditConferenceAlias, u.Uid, comm.Id, ipaddr, fmt.Sprintf("conf=%d", c.ConfId), fmt.Sprintf("add=%s", alias)))
	return nil
}

// RemoveAlias removes an alias from the conference.
func (c *Conference) RemoveAlias(ctx context.Context, alias string, u *User, comm *Community, ipaddr string) error {
	row := amdb.QueryRowContext(ctx, "SELECT COUNT(*) FROM confalias WHERE confid = ?", c.ConfId)
	aliasCount := 0
	err := row.Scan(&aliasCount)
	if err != nil {
		return err
	}

	if aliasCount == 1 {
		row = amdb.QueryRowContext(ctx, "SELECT alias FROM confalias WHERE confid = ? AND alias = ?", c.ConfId, alias)
		tmp := ""
		err = row.Scan(&tmp)
		if err == nil {
			return errors.New("the conference must have at least one alias")
		} else if err != sql.ErrNoRows {
			return err
		}
	}

	rs, err := amdb.ExecContext(ctx, "DELETE FROM confalias WHERE confid = ? AND alias = ?", c.ConfId, alias)
	if err != nil {
		return err
	}
	rowCount, err := rs.RowsAffected()
	if err != nil {
		return err
	}
	if rowCount != 1 {
		return errors.New("alias not found")
	}

	AmStoreAudit(AmNewCommAudit(AuditConferenceAlias, u.Uid, comm.Id, ipaddr, fmt.Sprintf("conf=%d", c.ConfId), fmt.Sprintf("remove=%s", alias)))
	return nil
}

// Hosts returns the list of users that host this conference.
func (c *Conference) Hosts(ctx context.Context) ([]*User, error) {
	var uids []int32
	err := amdb.SelectContext(ctx, &uids, "SELECT uid FROM confmember WHERE confid = ? AND granted_lvl = ?", c.ConfId, AmRole("Conference.Host").Level())
	if err != nil {
		return nil, err
	}
	rc := make([]*User, 0, len(uids))
	for _, uid := range uids {
		u, err := AmGetUser(ctx, uid)
		if err == nil {
			rc = append(rc, u)
		}
	}
	slices.SortFunc(rc, func(a, b *User) int {
		return strings.Compare(strings.ToLower(a.Username), strings.ToLower(b.Username))
	})
	return rc, nil
}

// Hosts returns the list of users that host this conference, quietly.
func (c *Conference) HostsQ(ctx context.Context) []*User {
	rc, _ := c.Hosts(ctx)
	return rc
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

// Members returns all the members of this conference, with their granted user levels.
func (c *Conference) Members(ctx context.Context) ([]ConferenceMember, error) {
	var rc []ConferenceMember
	err := amdb.SelectContext(ctx, &rc, "SELECT * FROM confmember WHERE confid = ?", c.ConfId)
	return rc, err
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

// SetMembership sets the membership level for the given user in this conference.
func (c *Conference) SetMembership(ctx context.Context, u *User, level uint16, by *User, comm *Community, ipaddr string) error {
	if level == 0 {
		_, err := amdb.ExecContext(ctx, "DELETE FROM confmember WHERE confid = ? AND uid = ?", c.ConfId, u.Uid)
		return err
	}
	row := amdb.QueryRowContext(ctx, "SELECT granted_lvl FROM confmember WHERE confid = ? AND uid = ?", c.ConfId, u.Uid)
	var oldLevel uint16
	err := row.Scan(&oldLevel)
	switch err {
	case nil:
		if oldLevel == level {
			return nil
		}
		_, err = amdb.ExecContext(ctx, "UPDATE confmember SET granted_lvl = ? WHERE confid = ? AND uid = ?", level, c.ConfId, u.Uid)
	case sql.ErrNoRows:
		_, err = amdb.ExecContext(ctx, "INSERT INTO confmember (confid, uid, granted_lvl) VALUES (?, ?, ?)", c.ConfId, u.Uid, level)
	}
	if err != nil {
		AmStoreAudit(AmNewCommAudit(AuditConferenceMembership, by.Uid, comm.Id, ipaddr, fmt.Sprintf("conf=%d", c.ConfId),
			fmt.Sprintf("uid=%d", u.Uid), fmt.Sprintf("level=%d", level)))
	}
	return err
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

// GetCustomBlocks gets the custom HTML blocks for the conference.
func (c *Conference) GetCustomBlocks(ctx context.Context) (string, string, error) {
	row := amdb.QueryRowContext(ctx, "SELECT htmltop, htmlbottom FROM confcustom WHERE confid = ?", c.ConfId)
	var topBlock, bottomBlock string
	err := row.Scan(&topBlock, &bottomBlock)
	switch err {
	case nil:
		return topBlock, bottomBlock, nil
	case sql.ErrNoRows:
		err = nil
	}
	return "", "", err
}

// SetCustomBlocks sets the custom HTML blocks for this conference.
func (c *Conference) SetCustomBlocks(ctx context.Context, topBlock, bottomBlock string) error {
	success := false
	tx := amdb.MustBegin()
	defer func() {
		if !success {
			tx.Rollback()
		}
	}()
	row := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM confcustom WHERE confid = ?", c.ConfId)
	ct := 0
	err := row.Scan(&ct)
	if err != nil {
		return err
	}
	if ct == 0 {
		_, err = tx.ExecContext(ctx, "INSERT INTO confcustom (confid, htmltop, htmlbottom) VALUES (?, ?, ?)", c.ConfId, topBlock, bottomBlock)
	} else {
		_, err = tx.ExecContext(ctx, "UPDATE confcustom SET htmltop = ?, htmlbottom = ? WHERE confid = ?", topBlock, bottomBlock, c.ConfId)
	}
	if err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	success = true
	return nil
}

// RemoveCustomBlocks removes the custom HTML blocks from this conference.
func (c *Conference) RemoveCustomBlocks(ctx context.Context) error {
	_, err := amdb.ExecContext(ctx, "DELETE FROM confcustom WHERE confid = ?", c.ConfId)
	return err
}

// ActivityReport is used to get activity reports from the conference or topic.
type ActivityReport struct {
	Uid      int32
	Username string
	LastRead *time.Time
	LastPost *time.Time
}

// Activity report types.
const (
	ActivityReportPosters = 0 // report on all posters
	ActivityReportReaders = 1 // report on all readers
)

/* GetActivity returns a list of ActivityReport objects detailing the conference activity.
 * Parameters:
 *     ctx - Standard Go context value.
 *     reportType - Determines which report to generate:
 *         ActivityReportPosters - Report on all posters in the conference.
 *         ActivityReportReaders - Report on all readers in the conference.
 * Returns:
 *     List of ActivityReport objects detailing the conference activity.
 *     Standard Go error status.
 */
func (c *Conference) GetActivity(ctx context.Context, reportType int) ([]ActivityReport, error) {
	var myfield string
	switch reportType {
	case ActivityReportPosters:
		myfield = "s.last_post"
	case ActivityReportReaders:
		myfield = "s.last_read"
	default:
		return nil, errors.New("invalid report type parameter")
	}
	sql := fmt.Sprintf(`SELECT s.uid, u.username, s.last_read, s.last_post FROM confsettings s, users u WHERE u.uid = s.uid 
		AND s.confid = ? AND u.is_anon = 0 AND ISNULL(%s) = 0 ORDER BY %s DESC`, myfield, myfield)
	rs, err := amdb.QueryContext(ctx, sql, c.ConfId)
	if err != nil {
		return nil, err
	}
	rc := make([]ActivityReport, 0)
	for rs.Next() {
		var cur ActivityReport
		err = rs.Scan(&(cur.Uid), &(cur.Username), &(cur.LastRead), &(cur.LastPost))
		if err != nil {
			return nil, err
		}
		rc = append(rc, cur)
	}
	return rc, nil
}

// Active user selection types.
const (
	ActiveUserReaders = 0 // select active readers
	ActiveUserPosters = 1 // select active posters
)

/* GetActiveUserEMailAddrs gets the E-mail addresses of each user that's active in the conference, omitting those that have opted out of mass E-mails.
 * Parameters:
 *     ctx - Standard Go context value.
 *     userSelect - Selects which type of users to return:
 *         ActiveUserReaders - Select users that have actively read.
 *         ActiveUserPosters - Select users that have actively posted.
 *     dayLimit - If less than 0, it is ignored. If equal to 0, this function is a no-op. Otherwise, specifies a limit on the number of days
 *                between the user's activity and now.
 * Returns:
 *     List of E-mail addresses matchin the criteria, in arbitrary order.
 *     Standard Go error status.
 */
func (c *Conference) GetActiveUserEMailAddrs(ctx context.Context, userSelect, dayLimit int) ([]string, error) {
	if dayLimit == 0 {
		return make([]string, 0), nil
	}
	var myfield string
	switch userSelect {
	case ActiveUserReaders:
		myfield = "s.last_read"
	case ActiveUserPosters:
		myfield = "s.last_post"
	default:
		return nil, errors.New("invalid user selection parameter")
	}
	sql := fmt.Sprintf(`SELECT c.email, %s FROM contacts c, users u, confsettings s, propuser p WHERE c.contactid = u.contactid AND u.uid = s.uid
		AND s.confid = ? AND u.is_anon = 0 AND u.uid = p.uid AND p.ndx = %d AND p.data NOT LIKE '%%%s%%' AND ISNULL(%s) = 0 ORDER BY %s DESC`, myfield, UserPropFlags,
		util.OptionCharFromIndex(UserFlagMassMailOptOut), myfield, myfield)
	rs, err := amdb.QueryContext(ctx, sql, c.ConfId)
	if err != nil {
		return nil, err
	}
	var stopPoint *time.Time = nil
	if dayLimit > 0 {
		mynow := time.Now().UTC()
		y, m, d := mynow.AddDate(0, 0, -dayLimit).Date()
		stopPointActual := time.Date(y, m, d, 0, 0, 0, 0, mynow.Location())
		stopPoint = &stopPointActual
	}
	rc := make([]string, 0)
	for rs.Next() {
		var addy string
		var point time.Time
		if err = rs.Scan(&addy, &point); err != nil {
			return nil, err
		}
		if stopPoint != nil && point.Before(*stopPoint) {
			break
		}
		rc = append(rc, addy)
	}
	return rc, nil
}

// Stats retrieves the number of topics and posts in this conference.
func (c *Conference) Stats(ctx context.Context) (int, int, error) {
	row := amdb.QueryRowContext(ctx, "SELECT COUNT(*), SUM(top_message + 1) FROM topics WHERE confid = ?", c.ConfId)
	ntopic := 0
	npost := 0
	err := row.Scan(&ntopic, &npost)
	return ntopic, npost, err
}

// backgroundPurgeConference purges out all the conference information in the background.
func backgroundPurgeConference(ctx context.Context, confid int32) error {
	// Purge out auxiliary conference tables first.
	tx := amdb.MustBegin()
	_, err := tx.ExecContext(ctx, "DELETE FROM confmember WHERE confid = ?", confid)
	if err != nil {
		log.Warnf("backgroundPurgeConference(%d): failed purging confmember: %v", confid, err)
	}
	_, err = tx.ExecContext(ctx, "DELETE FROM confsettings WHERE confid = ?", confid)
	if err != nil {
		log.Warnf("backgroundPurgeConference(%d): failed purging confsettings: %v", confid, err)
	}
	_, err = tx.ExecContext(ctx, "DELETE FROM confhotlist WHERE confid = ?", confid)
	if err != nil {
		log.Warnf("backgroundPurgeConference(%d): failed purging confhotlist: %v", confid, err)
	}
	_, err = tx.ExecContext(ctx, "DELETE FROM propconf WHERE confid = ?", confid)
	if err != nil {
		log.Warnf("backgroundPurgeConference(%d): failed purging propconf: %v", confid, err)
	}
	_, err = tx.ExecContext(ctx, "DELETE FROM confcustom WHERE confid = ?", confid)
	if err != nil {
		log.Warnf("backgroundPurgeConference(%d): failed purging confcustom: %v", confid, err)
	}
	err = tx.Commit()
	if err != nil {
		tx.Rollback()
		return err
	}

	// Get all topic IDs in this conference.
	var topicIds []int32
	err = amdb.SelectContext(ctx, &topicIds, "SELECT topicid FROM topics WHERE confid = ?", confid)
	if err != nil {
		return err
	}

	// Erase each topic in turn by calling two of the "delete topic" internal functions.
	for _, topicId := range topicIds {
		tx := amdb.MustBegin()
		err = eraseTopicRecords(ctx, tx, topicId)
		if err == nil {
			err = tx.Commit()
		}
		if err != nil {
			tx.Rollback()
			return err
		}
		err = backgroundPurgeTopic(ctx, topicId)
		if err != nil {
			return err
		}
	}
	return nil
}

// Delete unlinks this conference from the community, deleting it entirely if the last link is gone.
func (c *Conference) Delete(ctx context.Context, comm *Community, u *User, ipaddr string, background *util.WorkerPool) error {
	success := false
	tx := amdb.MustBegin()
	defer func() {
		if !success {
			tx.Rollback()
		}
	}()
	getConferenceMutex.Lock()
	defer getConferenceMutex.Unlock()

	// any references to conference other than this community?
	row := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM commtoconf WHERE confid = ? AND commid <> ?", c.ConfId, comm.Id)
	refCount := 0
	err := row.Scan(&refCount)
	if err != nil {
		return err
	}

	// break the link with the community
	if _, err = tx.ExecContext(ctx, "DELETE FROM commtoconf WHERE commid = ? AND confid = ?", comm.Id, c.ConfId); err != nil {
		return err
	}

	if refCount == 0 {
		// We have to delete all the conference core data now.
		_, err = tx.ExecContext(ctx, "DELETE FROM confs WHERE confid = ?", c.ConfId)
		if err == nil {
			_, err = tx.ExecContext(ctx, "DELETE FROM confalias WHERE confid = ?", c.ConfId)
		}
	}
	if err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return err
	}
	success = true

	if refCount == 0 {
		// kick the conference out of the cache
		conferenceCache.Remove(c.ConfId)

		// add an audit record
		AmStoreAudit(AmNewCommAudit(AuditConferenceDelete, u.Uid, comm.Id, ipaddr, fmt.Sprintf("confid=%d", c.ConfId)))

		// set up a background job to purge the rest of the data
		confid := c.ConfId
		background.Submit(func(ctx context.Context) {
			start := time.Now()
			// Just dump the whole conference property cache
			getConferencePropMutex.Lock()
			conferencePropCache.Purge()
			getConferencePropMutex.Unlock()

			// purge the conference data
			err := backgroundPurgeConference(ctx, confid)
			if err != nil {
				log.Errorf("Conference purge(#%d) background job failed: %v", confid, err)
			}
			dur := time.Since(start)
			log.Infof("Conference.Delete task completed in %v", dur)
		})
	}
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

/* AmGetConferenceContainingPost looks up a post ID and returns the conference containing it.
 * Parameters:
 *     ctx - Standard Go context value.
 *     postId - The post ID to look up.
 * Returns:
 *     Pointer to the conference, or nil.
 *     Standard Go error status.
 */
func AmGetConferenceContainingPost(ctx context.Context, postId int64) (*Conference, error) {
	row := amdb.QueryRowContext(ctx, "SELECT t.confid FROM topics t, posts p WHERE p.postid = ? AND p.topicid = t.topicid", postId)
	var confId int32
	err := row.Scan(&confId)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("post not found: %d", postId)
	} else if err != nil {
		return nil, err
	}
	return AmGetConference(ctx, confId)
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

/* AmListConferences returns all conferences for a given community.
 * Parameters:
 *     ctx - Standard Go context value.
 *     cid - Community ID to get conferences for.
 *     showHidden - true to show hidden conferences.
 * Returns:
 *     Array containing the ConferenceSummary pointers, or nil.
 *     Stanbard Go error status.
 */
func AmListConferences(ctx context.Context, cid int32, showHidden bool) ([]*ConferenceSummary, error) {
	q := ""
	if !showHidden {
		q = " AND x.hide_list = 0"
	}
	rs, err := amdb.QueryContext(ctx, `SELECT x.confid, c.name, c.lastupdate, c.descr, x.sequence, x.hide_list FROM commtoconf x, confs c
		WHERE x.confid = c.confid AND x.commid = ?`+q+" ORDER BY x.sequence, c.name", cid)
	if err != nil {
		return nil, err
	}
	rc := make([]*ConferenceSummary, 0)
	for rs.Next() {
		var cs ConferenceSummary
		if err = rs.Scan(&(cs.ConfId), &(cs.Name), &(cs.LastUpdate), &(cs.Description), &(cs.Sequence), &(cs.Hidden)); err == nil {
			rc = append(rc, &cs)
		} else {
			return nil, err
		}
	}
	for i := range rc {
		row := amdb.QueryRowContext(ctx, "SELECT alias FROM confalias WHERE confid = ?", rc[i].ConfId)
		err = row.Scan(&(rc[i].Alias))
		if err != nil {
			return nil, err
		}
		err = amdb.SelectContext(ctx, &(rc[i].Hosts), `SELECT u.username FROM confmember m, users u WHERE u.uid = m.uid AND m.confid = ?
			AND m.granted_lvl = ? ORDER BY u.username`, rc[i].ConfId, AmRole("Conference.Host").Level())
		if err != nil {
			return nil, err
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

// AmReorderConferences reorders two conferences by sequence number.
func AmReorderConferences(ctx context.Context, cid int32, seq1, seq2 int16) error {
	success := false
	tx := amdb.MustBegin()
	defer func() {
		if !success {
			tx.Rollback()
		}
	}()
	_, err := tx.ExecContext(ctx, "UPDATE commtoconf SET sequence = -1 WHERE commid = ? AND sequence = ?", cid, seq1)
	if err == nil {
		_, err = tx.ExecContext(ctx, "UPDATE commtoconf SET sequence = ? WHERE commid = ? AND sequence = ?", seq1, cid, seq2)
		if err == nil {
			_, err = tx.ExecContext(ctx, "UPDATE commtoconf SET sequence = ? WHERE commid = ? AND sequence = -1", seq2, cid)
		}
	}
	if err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	success = true
	return nil
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
	ar = AmNewCommAudit(AuditConferenceCreate, u.Uid, comm.Id, ipaddr, fmt.Sprintf("confid=%d", rc[0].ConfId), fmt.Sprintf("name=%s", name), fmt.Sprintf("alias=%s", alias))
	return &(rc[0]), nil
}

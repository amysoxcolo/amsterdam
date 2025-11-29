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
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"git.erbosoft.com/amy/amsterdam/util"
	lru "github.com/hashicorp/golang-lru"
	"github.com/jmoiron/sqlx"
	"golang.org/x/text/language"
)

// Community struct contains the high level data for a community.
type Community struct {
	Mutex             sync.RWMutex
	Id                int32      `db:"commid"`      // ID of the community
	CreateDate        time.Time  `db:"createdate"`  // timestamp for community creation
	LastAccess        *time.Time `db:"lastaccess"`  // timestamp for last access
	LastUpdate        *time.Time `db:"lastupdate"`  // timestamp for last update
	ReadLevel         uint16     `db:"read_lvl"`    // level required to read
	WriteLevel        uint16     `db:"write_lvl"`   // level required to write (change community attributes)
	CreateLevel       uint16     `db:"create_lvl"`  // level required to create subobjects
	DeleteLevel       uint16     `db:"delete_lvl"`  // level required to delete community
	JoinLevel         uint16     `db:"join_lvl"`    // level required to join
	ContactId         int32      `db:"contactid"`   // community's information as a contact info record ID
	HostUid           *int32     `db:"host_uid"`    // UID of the host
	CategoryId        int32      `db:"catid"`       // category ID for community
	HideFromDirectory bool       `db:"hide_dir"`    // if set, community is hidden from the directory
	HideFromSearch    bool       `db:"hide_search"` // if set, this community is hidden from search
	MembersOnly       bool       `db:"membersonly"` // is this community open to members only?
	IsAdmin           bool       `db:"is_admin"`    // set if this is the admin community
	InitFeature       int16      `db:"init_ftr"`    // initial feature?
	Name              string     `db:"commname"`    // community name
	Language          *string    `db:"language"`    // primary language of community, ISO format
	Synopsis          *string    `db:"synopsis"`    // community synopsis
	Rules             *string    `dd:"rules"`       // rules (kinda short)
	JoinKey           *string    `db:"joinkey"`     // join key (password) to join community
	Alias             string     `db:"alias"`       // community alias
	flags             *util.OptionSet
}

// CommunityProperties represents a property entry for a community.
type CommunityProperties struct {
	Cid   int32   `db:"cid"`  // community ID
	Index int32   `db:"ndx"`  // property index
	Data  *string `db:"data"` // property value
}

// Community property indexes defined.
const (
	CommunityPropFlags = int32(0) // "flags" user property
)

// Flag values for community property index CommunityPropFlags defined.
const (
	CommunityFlagPicturesInPosts = uint(0)
)

// Field and operation selectors for AmSearchCommunities.
const (
	SearchCommFieldName     = 0
	SearchCommFieldSynopsis = 1

	SearchCommOperPrefix    = 0
	SearchCommOperSubstring = 1
	SearchCommOperRegex     = 2
)

// Field and operator selectors for ListMembers.
const (
	ListMembersFieldNone        = -1
	ListMembersFieldName        = 0
	ListMembersFieldDescription = 1
	ListMembersFieldFirstName   = 2
	ListMembersFieldLastName    = 3

	ListMembersOperNone      = -1
	ListMembersOperPrefix    = 0
	ListMembersOperSubstring = 1
	ListMembersOperRegex     = 2
)

// communityCache is the cache for Community objects.
var communityCache *lru.TwoQueueCache = nil

// getCommunityMutex is a mutex on AmGetCommunity.
var getCommunityMutex sync.Mutex

// communityPropCache is the cache for CommunityProperties objects.
var communityPropCache *lru.Cache = nil

// getCommunityPropMutex is a mutex on AmGetCommunityProperty.
var getCommunityPropMutex sync.Mutex

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

// init initializes the caches.
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
	communityPropCache, err = lru.New(100)
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

// HostQ returns the reference to the community's host, quietly.
func (c *Community) HostQ() *User {
	if c.HostUid == nil {
		return nil
	}
	u, err := AmGetUser(*c.HostUid)
	if err != nil {
		return nil
	}
	return u
}

// LanguageTag returns the tag for the community's language.
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

// MemberCount returns the number of members in the community.
func (c *Community) MemberCount(hidden bool) (int, error) {
	var rs *sql.Rows
	var err error
	if hidden {
		rs, err = amdb.Query("SELECT COUNT(*) FROM commmember WHERE commid = ?", c.Id)
	} else {
		rs, err = amdb.Query("SELECT COUNT(*) FROM commmember WHERE commid = ? AND hidden = 0", c.Id)
	}
	if err != nil {
		return -1, err
	}
	if rs.Next() {
		var rc int
		rs.Scan(&rc)
		return rc, nil
	}
	return -1, errors.New("internal error reading member count")
}

/* ListMembers lists or searches for community members matching certain criteria.
 * Parameters:
 *     field - A value indicating which field to search:
 *         ListMembersFieldNone - Do not search, just return all community members.
 *         ListMembersFieldName - The user name.
 *         ListMembersFieldDescription - The user description.
 *         ListMembersFieldFirstName - The user's first name.
 *         ListMembersFieldLastName - The user's last name.
 *     oper - The operation to perform on the search field:
 *         ListMembersOperNone - Do not search, just return all community members.
 *         ListMembersOperPrefix - The specified field has the string "term" as a prefix.
 *         ListMembersOperSubstring - The specified field contains the string "term".
 *         ListMembersOperRegex - The specified field matches the regular expression in "term".
 *     term - The search term, as specified above.
 *     offset - Number of members to skip at beginning of list.
 *     max - Maximum number of members to return.
 * Returns:
 *     Array of User pointers representing the return elements.
 *     The total number of members matching this query (could be greater than max)
 *	   Standard Go error status.
 */
func (c *Community) ListMembers(field int, oper int, term string, offset int, max int, showHidden bool) ([]*User, int, error) {
	var query strings.Builder
	if field != ListMembersFieldNone && oper != ListMembersOperNone {
		query.WriteString(" AND ")
		switch field {
		case ListMembersFieldName:
			query.WriteString("u.username ")
		case ListMembersFieldDescription:
			query.WriteString("u.description ")
		case ListMembersFieldFirstName:
			query.WriteString("c.given_name ")
		case ListMembersFieldLastName:
			query.WriteString("c.family_name ")
		default:
			return nil, -1, errors.New("invalid field selector")
		}
		switch oper {
		case ListMembersOperPrefix:
			query.WriteString("LIKE '")
			query.WriteString(util.SqlEscape(term, true))
			query.WriteString("%'")
		case ListMembersOperSubstring:
			query.WriteString("LIKE '%")
			query.WriteString(util.SqlEscape(term, true))
			query.WriteString("%'")
		case ListMembersOperRegex:
			query.WriteString("REGEXP '")
			query.WriteString(util.SqlEscape(term, false))
			query.WriteString("'")
		default:
			return nil, -1, errors.New("invalid operator selector")
		}
	}
	if !showHidden {
		query.WriteString(" AND m.hidden = 0")
	}
	q := query.String()
	rs, err := amdb.Query(`SELECT COUNT(*) FROM commmember m, users u, contacts c WHERE m.commid = ? AND m.uid = u.uid
		AND u.contactid = c.contactid`+q, c.Id)
	if err != nil {
		return nil, -1, err
	}
	if !rs.Next() {
		return nil, -1, errors.New("internal error getting member count")
	}
	var total int
	rs.Scan(&total)
	if offset > 0 {
		rs, err = amdb.Query(`SELECT m.uid FROM commmember m, users u, contacts c WHERE m.commid = ? AND m.uid = u.uid
			AND u.contactid = c.contactid`+q+" ORDER BY u.username LIMIT ? OFFSET ?", c.Id, max, offset)
	} else {
		rs, err = amdb.Query(`SELECT m.uid FROM commmember m, users u, contacts c WHERE m.commid = ? AND m.uid = u.uid
			AND u.contactid = c.contactid`+q+" ORDER BY u.username LIMIT ?", c.Id, max)
	}
	if err != nil {
		return nil, total, err
	}
	rc := make([]*User, 0, min(max, 10000))
	for rs.Next() {
		var uid int32
		rs.Scan(&uid)
		u, err := AmGetUser(uid)
		if err == nil {
			rc = append(rc, u)
		}
	}
	return rc, total, nil
}

/* SetMembership sets a user's membership status within the community.
 * Parameters:
 *     u - The user to change the membership status of.
 *     level - Their membership level. If this is 0, they are removed from membership.
 *     locked - Whether they can unjoin the community themselves. Ignored if removing them.
 *     personUID - The UID of the person taking this action.
 *     ipaddr - The source IP address, for audit records.
 * Returns:
 *     Standard Go error status.
 */
func (c *Community) SetMembership(u *User, level uint16, locked bool, personUID int32, ipaddr string) error {
	success := false
	tx := amdb.MustBegin()
	defer func() {
		if !success {
			tx.Rollback()
		}
	}()
	if level == 0 {
		res, err := tx.Exec("DELETE FROM commmember WHERE commid = ? AND uid = ?", c.Id, u.Uid)
		if err != nil {
			return err
		}
		stuffMembership(c.Id, u.Uid, false, false, 0)
		ra, err := res.RowsAffected()
		if err == nil && ra > 0 {
			err = AmOnUserLeaveCommunityServices(tx, c, u)
			if err != nil {
				return err
			}
		}
	} else {
		rs, err := tx.Query("SELECT granted_lvl, locked FROM commmember WHERE commid = ? AND uid = ?", c.Id, u.Uid)
		if err != nil {
			return err
		}
		if rs.Next() {
			var oldLevel uint16
			var lockStatus bool
			rs.Scan(&oldLevel, &lockStatus)
			if level != oldLevel || lockStatus != locked {
				_, err := tx.Exec("UPDATE commmember SET granted_lvl = ?, locked = ? WHERE commid = ? AND uid = ?",
					level, locked, c.Id, u.Uid)
				if err != nil {
					return err
				}
				stuffMembership(c.Id, u.Uid, true, locked, level)
			}
		} else {
			_, err := tx.Exec("INSERT INTO commmember (commid, uid, granted_lvl, locked) VALUES (?, ?, ?, ?)",
				c.Id, u.Uid, level, locked)
			if err != nil {
				return err
			}
			stuffMembership(c.Id, u.Uid, true, locked, level)
			err = AmOnUserJoinCommunityServices(tx, c, u)
			if err != nil {
				return err
			}
		}
	}
	err := c.TouchUpdateTx(tx)
	if err == nil {
		ar := AmNewAudit(AuditCommunitySetMembership, personUID, ipaddr, fmt.Sprintf("cid=%d", c.Id),
			fmt.Sprintf("uid=%d", u.Uid), fmt.Sprintf("level=%d", level))
		AmStoreAudit(ar)
	}
	return nil
}

/* TestPermission is shorthand that tests if a user has a permission with respect to the community.
 * Parameters:
 *     user - The user to be checked.
 *     perm - The permission to be tested.
 * Returns:
 *     true if the user has the permission, false if not.
 *     Standard Go error status.
 */
func (c *Community) TestPermission(perm string, level uint16) bool {
	switch perm {
	case "Community.Read":
		return level >= c.ReadLevel
	case "Community.Write":
		return level >= c.WriteLevel
	case "Community.Create":
		return level >= c.CreateLevel
	case "Community.Delete":
		return level >= c.DeleteLevel
	case "Community.Join":
		return level >= c.JoinLevel
	default:
		return AmTestPermission(perm, level)
	}
}

// PermissionLevel returns the permission level for a permission name.
func (c *Community) PermissionLevel(perm string) uint16 {
	switch perm {
	case "Community.Read":
		return c.ReadLevel
	case "Community.Write":
		return c.WriteLevel
	case "Community.Create":
		return c.CreateLevel
	case "Community.Delete":
		return c.DeleteLevel
	case "Community.Join":
		return c.JoinLevel
	default:
		return AmPermissionLevel(perm)
	}
}

// GetFlags retrieves the flags from the properties.
func (c *Community) Flags() (*util.OptionSet, error) {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()
	if c.flags == nil {
		s, err := AmGetCommunityProperty(c.Id, CommunityPropFlags)
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
func (c *Community) SaveFlags(f *util.OptionSet) error {
	s := f.AsString()
	c.Mutex.Lock()
	defer c.Mutex.Unlock()
	err := AmSetCommunityProperty(c.Id, CommunityPropFlags, &s)
	if err == nil {
		c.flags = f
	}
	return err
}

// SetProfileData sets all the "settable" profile data
func (c *Community) SetProfileData(name string, alias string, synopsis *string, rules *string, language *string,
	joinkey *string, membersonly bool, hideDirectory bool, hideSearch bool, read_lvl uint16, write_lvl uint16,
	create_lvl uint16, delete_lvl uint16, join_lvl uint16) error {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()
	_, err := amdb.Exec(`UPDATE communities SET commname = ?, alias = ?, synopsis = ?, rules = ?, language = ?,
		joinkey = ?, membersonly = ?, hide_dir = ?, hide_search = ?, read_lvl = ?, write_lvl = ?, create_lvl = ?,
		delete_lvl = ?, join_lvl = ?, lastupdate = NOW() WHERE commid = ?`,
		name, alias, synopsis, rules, language, joinkey, membersonly, hideDirectory, hideSearch, read_lvl, write_lvl,
		create_lvl, delete_lvl, join_lvl, c.Id)
	if err == nil {
		c.Name = name
		c.Alias = alias
		c.Synopsis = synopsis
		c.Rules = rules
		c.Language = language
		c.JoinKey = joinkey
		c.MembersOnly = membersonly
		c.HideFromDirectory = hideDirectory
		c.HideFromSearch = hideSearch
		c.ReadLevel = read_lvl
		c.WriteLevel = write_lvl
		c.CreateLevel = create_lvl
		c.DeleteLevel = delete_lvl
		c.JoinLevel = join_lvl
		rs, err2 := amdb.Query("SELECT lastupdate FROM communities WHERE commid = ?", c.Id)
		if err2 != nil {
			rs.Next()
			rs.Scan(&c.LastUpdate)
		}
	}
	return err
}

// SetContactID sets the contact ID for the community.
func (c *Community) SetContactID(cid int32) error {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()
	if _, err := amdb.Exec("UPDATE communities SET contactid = ? WHERE commid = ?", cid, c.Id); err != nil {
		return err
	}
	c.ContactId = cid
	return nil
}

// Touch updates the last access time of the community.
func (c *Community) Touch() error {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()
	_, err := amdb.Exec("UPDATE communities SET lastaccess = NOW() WHERE commid = ?", c.Id)
	if err == nil {
		rs, err := amdb.Query("SELECT lastaccess FROM communities WHERE commid = ?", c.Id)
		if err == nil {
			rs.Next()
			var na time.Time
			rs.Scan(&na)
			c.LastAccess = &na
		}
	}
	return err
}

// TouchUpdateTx updates the last access and last update times of the community.
func (c *Community) TouchUpdateTx(tx *sqlx.Tx) error {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()
	_, err := tx.Exec("UPDATE communities SET lastaccess = NOW(), lastupdate = NOW() WHERE commid = ?", c.Id)
	if err == nil {
		rs, err := tx.Query("SELECT lastaccess, lastupdate FROM communities WHERE commid = ?", c.Id)
		if err != nil {
			rs.Next()
			var na, nu time.Time
			rs.Scan(&na, &nu)
			c.LastAccess = &na
			c.LastUpdate = &nu
		}
	}
	return err
}

// TouchUpdateTx updates the last access and last update times of the community.
func (c *Community) TouchUpdate() error {
	tx := amdb.MustBegin()
	err := c.TouchUpdateTx(tx)
	if err != nil {
		err = tx.Commit()
	}
	if err != nil {
		tx.Rollback()
	}
	return err
}

/* AmGetCommunity returns a reference to the specified community.
 * Parameters:
 *     id - The ID of the community.
 * Returns:
 *     Pointer to Community containing community data, or nil
 *     Standard Go error status
 */
func AmGetCommunity(id int32) (*Community, error) {
	getCommunityMutex.Lock()
	defer getCommunityMutex.Unlock()
	rc, ok := communityCache.Get(id)
	if !ok {
		var dbdata []Community
		err := amdb.Select(&dbdata, "SELECT * from communities WHERE commid = ?", id)
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
	return rc.(*Community), nil
}

/* AmGetCommunityTx returns a reference to the specified community, in a transaction.
 * Parameters:
 *     tx - The transaction to use.
 *     id - The ID of the community.
 * Returns:
 *     Pointer to Community containing community data, or nil
 *     Standard Go error status
 */
func AmGetCommunityTx(tx *sqlx.Tx, id int32) (*Community, error) {
	getCommunityMutex.Lock()
	defer getCommunityMutex.Unlock()
	rc, ok := communityCache.Get(id)
	if !ok {
		var dbdata []Community
		err := tx.Select(&dbdata, "SELECT * from communities WHERE commid = ?", id)
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
	return rc.(*Community), nil
}

/* AmGetCommunityByAlias returns a reference to the specified community.
 * Parameters:
 *     alias - The alias for the community.
 * Returns:
 *     Pointer to Community containing community data, or nil
 *     Standard Go error status (nil if community not found)
 */
func AmGetCommunityByAlias(alias string) (*Community, error) {
	rs, err := amdb.Query("SELECT commid FROM communities WHERE alias = ?", alias)
	if err == nil {
		if rs.Next() {
			var cid int32
			rs.Scan(&cid)
			return AmGetCommunity(cid)
		} else {
			return nil, nil
		}
	}
	return nil, err
}

/* AmGetCommunityByAliasTx returns a reference to the specified community, within a transaction.
 * Parameters:
 *     tx - The transaction to use.
 *     alias - The alias for the community.
 * Returns:
 *     Pointer to Community containing community data, or nil
 *     Standard Go error status (nil if community not found)
 */
func AmGetCommunityByAliasTx(tx *sqlx.Tx, alias string) (*Community, error) {
	rs, err := tx.Query("SELECT commid FROM communities WHERE alias = ?", alias)
	if err == nil {
		if rs.Next() {
			var cid int32
			rs.Scan(&cid)
			return AmGetCommunityTx(tx, cid)
		} else {
			return nil, nil
		}
	}
	return nil, err
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
	rc, err := AmGetCommunityByAlias(param)
	if err == nil {
		if rc == nil {
			return nil, fmt.Errorf("community with alias \"%s\" not found", param)
		}
	}
	return rc, err
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
 *     tx - The current transaction to be used for database access.
 *     user - The user to be auto-joined to communities.
 * Returns:
 *     Standard Go error status.
 */
func AmAutoJoinCommunities(tx *sqlx.Tx, user *User) error {
	// get list of current communities
	var current []int32 = make([]int32, 0)
	err := tx.Select(&current, "SELECT commid FROM commmember WHERE uid = ?", user.Uid)
	if err != nil {
		return err
	}

	// look for candidate communities
	rows, err := tx.Queryx(`SELECT m.commid, m.locked FROM users u, communities c, commmember m
		WHERE m.uid = u.uid AND m.commid = c.commid AND u.is_anon = 1 AND c.join_lvl <= ?`, user.BaseLevel)
	if err == nil {
		defer rows.Close()
		grantLevel := AmDefaultRole("Community.NewUser").Level()
		for rows.Next() {
			var cid int32
			var lock bool
			rows.Scan(&cid, &lock)
			if !slices.Contains(current, cid) {
				_, err = tx.Exec("INSERT INTO commmember (commid, uid, granted_lvl, locked) VALUES (?, ?, ?, ?)",
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

// internalGetProp is a helper used by the property functions.
func internalGetCommProp(cid int32, ndx int32) (*CommunityProperties, error) {
	var err error = nil
	key := fmt.Sprintf("%d:%d", cid, ndx)
	getCommunityPropMutex.Lock()
	defer getCommunityPropMutex.Unlock()
	rc, ok := communityPropCache.Get(key)
	if !ok {
		var dbdata []CommunityProperties
		err = amdb.Select(&dbdata, "SELECT * from propcomm WHERE cid = ? AND ndx = ?", cid, ndx)
		if err != nil {
			return nil, err
		}
		if len(dbdata) == 0 {
			return nil, nil
		}
		if len(dbdata) > 1 {
			return nil, fmt.Errorf("AmGetCommunityProperty(%d): too many responses(%d)", cid, len(dbdata))
		}
		rc = &(dbdata[0])
		communityPropCache.Add(key, rc)
	}
	return rc.(*CommunityProperties), nil
}

/* AmGetCommunityProperty retrieves the value of a user property.
 * Parameters:
 *     cid - The ID of the community to get the property for.
 *     ndx - The index of the property to retrieve.
 * Returns:
 *     Value of the property string.
 *     Standard Go error status.
 */
func AmGetCommunityProperty(cid int32, ndx int32) (*string, error) {
	p, err := internalGetCommProp(cid, ndx)
	if err != nil {
		return nil, err
	} else if p == nil {
		return nil, nil
	}
	return p.Data, nil
}

/* AmSetCommunityProperty sets the value of a community property.
 * Parameters:
 *     cid - The ID of the community to set the property for.
 *     ndx - The index of the property to set.
 *     val - The new value of the property.
 * Returns:
 *     Standard Go error status.
 */
func AmSetCommunityProperty(cid int32, ndx int32, val *string) error {
	p, err := internalGetCommProp(cid, ndx)
	if err != nil {
		return err
	}
	getCommunityPropMutex.Lock()
	defer getCommunityPropMutex.Unlock()
	if p != nil {
		_, err = amdb.Exec("UPDATE propcomm SET data = ? WHERE cid = ? AND ndx = ?", val, cid, ndx)
		if err == nil {
			p.Data = val
		}
	} else {
		prop := CommunityProperties{Cid: cid, Index: ndx, Data: val}
		_, err := amdb.NamedExec("INSERT INTO propcomm (cid, ndx, data) VALUES(:cid, :ndx, :data)", prop)
		if err == nil {
			communityPropCache.Add(fmt.Sprintf("%d:%d", cid, ndx), prop)
		}
	}
	return err
}

/* AmCreateCommunity creates a new community.
 * Parameters:
 *     name - The name for the new community.
 *     alias - The alias for the new community. Must be unique.
 *     hostUid - The UID of the creator and new host of the community.
 *     language - Community default language.
 *     synopsis - Community synopsis string.
 *     rules - Community rules string.
 *     joinkey - Community join key, or empty string for a public community.
 *     hideDirectory - true to hide this community from the directory listings.
 *     hideSearch - true to hide this community from searches.
 *     remoteIP - Remote IP address for audit record.
 * Returns:
 *     Pointer to new Community record, or nil.
 *     Standard Go error status.
 */
func AmCreateCommunity(name string, alias string, hostUid int32, language *string, synopsis *string,
	rules *string, joinkey *string, hideDirectory bool, hideSearch bool, remoteIP string) (*Community, error) {
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

	// validate alias does not already exist
	rs, err := tx.Query("SELECT commid FROM communities WHERE alias = ?", alias)
	if err != nil {
		return nil, err
	}
	if rs.Next() {
		return nil, errors.New("a community with that alias already exists")
	}

	// establish the community record
	_, err = tx.Exec(`INSERT INTO communities (createdate, lastaccess, lastupdate, read_lvl, write_lvl,
		create_lvl, delete_lvl, join_lvl, host_uid, hide_dir, hide_search, commname, language,
		synopsis, rules, joinkey, alias) VALUES (NOW(), NOW(), NOW(), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		AmRoleList("Community.Read").Default().Level(), AmRoleList("Community.Write").Default().Level(),
		AmRoleList("Community.Create").Default().Level(), AmRoleList("Community.Delete").Default().Level(),
		AmRoleList("Community.Join").Default().Level(), hostUid, hideDirectory, hideSearch, name, language,
		synopsis, rules, joinkey, alias)
	if err != nil {
		return nil, err
	}

	// Read back the community, which also puts it in the cache.
	comm, err := AmGetCommunityByAliasTx(tx, alias)
	if err != nil {
		return nil, err
	} else if comm == nil {
		return nil, errors.New("unable to find newly-generated community")
	}

	// Ensure the new host has host privileges in the community. The host's membership is "locked" so they
	// can't unjoin and leave the community hostless.
	_, err = tx.Exec("INSERT INTO commmember (commid, uid, granted_lvl, locked) VALUES (?, ?, ?, 1)", comm.Id, hostUid,
		AmDefaultRole("Community.Creator").Level())
	if err != nil {
		return nil, err
	}
	stuffMembership(comm.Id, hostUid, true, true, AmDefaultRole("Community.Creator").Level())

	// Establish the community services.
	err = AmEstablishCommunityServices(tx, comm)
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}
	success = true

	// operation was a success - add an audit record
	ar = AmNewAudit(AuditCommunityCreate, hostUid, remoteIP, fmt.Sprintf("id=%d", comm.Id),
		fmt.Sprintf("name=%s", comm.Name), fmt.Sprintf("alias=%s", comm.Alias))
	return comm, nil
}

/* AmGetCommunitiesForCategory returns a list of communities for the specified category.
 * Parameters:
 *     catid - Category ID to search for.
 *     offset - Number of communities to skip at beginning of list.
 *     max - Maximum number of communities to return.
 *     showAll - Include communities that are "hidden in directory."
 * Returns:
 *     Array of Community pointers representing the return elements.
 *     The total number of communities matching this query (could be greater than max)
 *	   Standard Go error status.
 */
func AmGetCommunitiesForCategory(catid int32, offset int, max int, showAll bool) ([]*Community, int, error) {
	var rs *sql.Rows
	var err error
	if showAll {
		rs, err = amdb.Query("SELECT COUNT(*) FROM communities WHERE catid = ?", catid)
	} else {
		rs, err = amdb.Query("SELECT COUNT(*) FROM communities WHERE catid = ? AND hide_dir = 0", catid)
	}
	if err != nil {
		return nil, -1, err
	}
	if !rs.Next() {
		return nil, -1, errors.New("internal error getting total match count")
	}
	var total int
	rs.Scan(&total)
	if total == 0 {
		return make([]*Community, 0), 0, nil // short-circuit return
	}
	if showAll {
		if offset > 0 {
			rs, err = amdb.Query("SELECT commid FROM communities WHERE catid = ? ORDER BY commname LIMIT ? OFFSET ?",
				catid, max, offset)
		} else {
			rs, err = amdb.Query("SELECT commid FROM communities WHERE catid = ? ORDER BY commname LIMIT ?", catid, max)
		}
	} else {
		if offset > 0 {
			rs, err = amdb.Query("SELECT commid FROM communities WHERE catid = ? AND hide_dir = 0 ORDER BY commname LIMIT ? OFFSET ?",
				catid, max, offset)
		} else {
			rs, err = amdb.Query("SELECT commid FROM communities WHERE catid = ? AND hide_dir = 0 ORDER BY commname LIMIT ?", catid, max)
		}
	}
	if err != nil {
		return nil, total, err
	}
	rc := make([]*Community, 0, min(max, 10000))
	for rs.Next() {
		var commid int32
		rs.Scan(&commid)
		c, err := AmGetCommunity(commid)
		if err == nil {
			rc = append(rc, c)
		}
	}
	return rc, total, nil
}

/* AmSearchCommunities searches for communities matching certain criteria.
 * Parameters:
 *     field - A value indicating which field to search:
 *         SearchCommFieldName - The community name.
 *         SearchCommFieldSynopsis - The communty synopsis.
 *     oper - The operation to perform on the search field:
 *         SearchCommOperPrefix - The specified field has the string "term" as a prefix.
 *         SearchCommOperSubstring - The specified field contains the string "term".
 *         SearchCommOperRegex - The specified field matches the regular expression in "term".
 *     term - The search term, as specified above.
 *     offset - Number of communities to skip at beginning of list.
 *     max - Maximum number of communities to return.
 *     showAll - Include communities that are "hidden in search."
 * Returns:
 *     Array of Community pointers representing the return elements.
 *     The total number of communities matching this query (could be greater than max)
 *	   Standard Go error status.
 */
func AmSearchCommunities(field int, oper int, term string, offset int, max int, showAll bool) ([]*Community, int, error) {
	var queryPortion strings.Builder
	queryPortion.WriteString("WHERE ")
	switch field {
	case SearchCommFieldName:
		queryPortion.WriteString("commname ")
	case SearchCommFieldSynopsis:
		queryPortion.WriteString("synopsis ")
	default:
		return nil, -1, errors.New("invalid field selector")
	}
	switch oper {
	case SearchCommOperPrefix:
		queryPortion.WriteString("LIKE '")
		queryPortion.WriteString(util.SqlEscape(term, true))
		queryPortion.WriteString("%'")
	case SearchCommOperSubstring:
		queryPortion.WriteString("LIKE '%")
		queryPortion.WriteString(util.SqlEscape(term, true))
		queryPortion.WriteString("%'")
	case SearchCommOperRegex:
		queryPortion.WriteString("REGEXP '")
		queryPortion.WriteString(util.SqlEscape(term, false))
		queryPortion.WriteString("'")
	default:
		return nil, -1, errors.New("invalid operator selector")
	}
	if !showAll {
		queryPortion.WriteString(" AND hide_search = 0")
	}
	q := queryPortion.String()
	rs, err := amdb.Query("SELECT COUNT(*) FROM communities " + q)
	if err != nil {
		return nil, -1, err
	}
	if !rs.Next() {
		return nil, -1, errors.New("internal error getting count")
	}
	var total int
	rs.Scan(&total)
	if total == 0 {
		return make([]*Community, 0), 0, nil // short-circuit return
	}
	if offset > 0 {
		rs, err = amdb.Query("SELECT commid FROM communities "+q+" ORDER BY commname LIMIT ? OFFSET ?", max, offset)
	} else {
		rs, err = amdb.Query("SELECT commid FROM communities "+q+" ORDER BY commname LIMIT ?", max)
	}
	if err != nil {
		return nil, total, err
	}
	rc := make([]*Community, 0, min(max, 10000))
	for rs.Next() {
		var commid int32
		rs.Scan(&commid)
		c, err := AmGetCommunity(commid)
		if err == nil {
			rc = append(rc, c)
		}
	}
	return rc, total, nil
}

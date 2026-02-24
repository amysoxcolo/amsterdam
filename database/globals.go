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
	"sync"

	"git.erbosoft.com/amy/amsterdam/util"
)

// Globals contains the global data.
type Globals struct {
	Mutex                   sync.Mutex
	PostsPerPage            int32 `db:"posts_per_page"`
	OldPostsAtTop           int32 `db:"old_posts_at_top"`
	MaxSearchPage           int32 `db:"max_search_page"`
	MaxCommunityMemberPage  int32 `db:"max_comm_mbr_page"`
	MaxConferenceMemberPage int32 `db:"max_conf_mbr_page"`
	FrontPagePosts          int32 `db:"fp_posts"`
	NumAuditPage            int32 `db:"num_audit_page"`
	CommunityCreateLevel    int32 `db:"comm_create_lvl"`
	flags                   *util.OptionSet
}

// GlobalProperties contains global property entries.
type GlobalProperties struct {
	Index int32  `db:"ndx"`
	Data  string `db:"data"`
}

// Global property indexes defined.
const (
	GlobalPropFlags = int32(0)
)

// Global flag indexes defined.
const (
	GlobalFlagPicturesInPosts = uint(0)
	GlobalFlagNoCategories    = uint(1)
)

// theGlobals contains the singleton instance of Globals.
var theGlobals *Globals = nil

// globalsMutex controls access to theGlobals.
var globalsMutex sync.Mutex

// globalProps is the global properties store.
var globalProps map[int32]string = make(map[int32]string)

// globalPropMutex controls access to globalProps.
var globalPropMutex sync.Mutex

// Clone clones the entire global state.
func (g *Globals) Clone() *Globals {
	rc := Globals{
		PostsPerPage:            g.PostsPerPage,
		OldPostsAtTop:           g.OldPostsAtTop,
		MaxSearchPage:           g.MaxSearchPage,
		MaxCommunityMemberPage:  g.MaxCommunityMemberPage,
		MaxConferenceMemberPage: g.MaxConferenceMemberPage,
		FrontPagePosts:          g.FrontPagePosts,
		NumAuditPage:            g.NumAuditPage,
		CommunityCreateLevel:    g.CommunityCreateLevel,
		flags:                   nil,
	}
	return &rc
}

// Flags returns the global flags.
func (g *Globals) Flags(ctx context.Context) (*util.OptionSet, error) {
	g.Mutex.Lock()
	defer g.Mutex.Unlock()
	if g.flags == nil {
		s, err := AmGetGlobalProperty(ctx, GlobalPropFlags)
		if err != nil {
			return nil, err
		}
		g.flags = util.OptionSetFromString(s)
	}
	return g.flags, nil
}

// SaveFlags saves off the global flags.
func (g *Globals) SaveFlags(ctx context.Context, f *util.OptionSet) error {
	s := f.AsString()
	g.Mutex.Lock()
	defer g.Mutex.Unlock()
	err := AmSetGlobalProperty(ctx, GlobalPropFlags, s)
	if err == nil {
		g.flags = f
	}
	return err
}

// AmGlobals returns trhe pointer to the singleton Globals instance.
func AmGlobals(ctx context.Context) (*Globals, error) {
	globalsMutex.Lock()
	defer globalsMutex.Unlock()
	if theGlobals == nil {
		var g Globals
		if err := amdb.GetContext(ctx, &g, "SELECT * FROM globals"); err != nil {
			return nil, err
		}
		theGlobals = &g
	}
	return theGlobals, nil
}

// AmReplaceGlobals writes the globals to the database and replaces the instance.
func AmReplaceGlobals(ctx context.Context, ng *Globals) error {
	globalsMutex.Lock()
	defer globalsMutex.Unlock()
	_, err := amdb.NamedExecContext(ctx, `UPDATE globals SET posts_per_page = :posts_per_page, old_posts_at_top = :old_posts_at_top, max_search_page = :max_search_page,
		max_comm_mbr_page = :max_comm_mbr_page, max_conf_mbr_page = :max_conf_mbr_page, fp_posts = :fp_posts, num_audit_page = :num_audit_page,
		comm_create_lvl = :comm_create_lvl`, ng)
	if err != nil {
		return err
	}
	ng.flags = nil
	theGlobals = ng
	return nil
}

/* AmGetGlobalProperty returns the value of a global property.
 * Parameters:
 *     ctx - Standard Go context value.
 *     index - The index of the property to retrieve.
 * Returns:
 *     Value of the property, or empty string.
 *     Standard Go error status.
 */
func AmGetGlobalProperty(ctx context.Context, index int32) (string, error) {
	globalPropMutex.Lock()
	defer globalPropMutex.Unlock()
	var err error = nil
	rc, ok := globalProps[index]
	if !ok {
		err := amdb.GetContext(ctx, &rc, "SELECT data FROM propglobal WHERE ndx = ?", index)
		switch err {
		case nil:
			globalProps[index] = rc
		case sql.ErrNoRows:
			rc = ""
			err = nil
		}
	}
	return rc, err
}

/* AmSetGlobalProperty sets the value of a global property.
 * Parameters:
 *     ctx - Standard Go context value.
 *     index - The index of the property to set.
 *     value - The value of the property to set.
 * Returns:
 *     Standard Go error status.
 */
func AmSetGlobalProperty(ctx context.Context, index int32, value string) error {
	globalPropMutex.Lock()
	defer globalPropMutex.Unlock()
	_, updateMode := globalProps[index]
	if !updateMode {
		var tmpdata string
		err := amdb.GetContext(ctx, &tmpdata, "SELECT data FROM propglobal WHERE ndx = ?", index)
		switch err {
		case nil:
			updateMode = true
		case sql.ErrNoRows:
			updateMode = false
		default:
			return err
		}
	}
	var err error = nil
	if updateMode {
		_, err = amdb.ExecContext(ctx, "UPDATE propglobal SET data = ? WHERE ndx = ?", value, index)
	} else {
		_, err = amdb.ExecContext(ctx, "INSERT INTO propglobal (ndx, data) VALUES (?, ?)", index, value)
	}
	if err == nil {
		globalProps[index] = value
	}
	return err
}

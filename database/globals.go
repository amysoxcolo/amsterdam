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
		var dbdata []Globals
		err := amdb.SelectContext(ctx, &dbdata, "SELECT * FROM globals")
		if err != nil {
			return nil, err
		}
		if len(dbdata) > 1 {
			return nil, errors.New("should only be one globals record")
		}
		theGlobals = &(dbdata[0])
	}
	return theGlobals, nil
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
	rc, ok := globalProps[index]
	if !ok {
		rs, err := amdb.QueryContext(ctx, "SELECT data FROM propglobal WHERE ndx = ?", index)
		if err != nil {
			return "", err
		}
		if rs.Next() {
			err = rs.Scan(&rc)
			if err != nil {
				return "", err
			}
			globalProps[index] = rc
			return rc, nil
		}
		rc = ""
	}
	return rc, nil
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
		rs, err := amdb.QueryContext(ctx, "SELECT data FROM propglobal WHERE ndx = ?", index)
		if err != nil {
			return err
		}
		updateMode = rs.Next()
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

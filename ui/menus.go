/*
 * Amsterdam Web Communities System
 * Copyright (c) 2025-2026 Erbosoft Metaverse Design Solutions, All Rights Reserved
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */
// Package ui holds the support for the Amsterdam user interface, wrapping Echo and Jet templates.
package ui

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"

	"git.erbosoft.com/amy/amsterdam/config"
	"git.erbosoft.com/amy/amsterdam/database"
	"git.erbosoft.com/amy/amsterdam/util"
	lru "github.com/hashicorp/golang-lru"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// MenuItem represents an item within a menu definition.
type MenuItem struct {
	Text       string `yaml:"text"`
	Link       string `yaml:"link"`
	Image      string `yaml:"image"`
	Disabled   bool   `yaml:"disabled"`
	Hazard     bool   `yaml:"hazard"`
	Permission string `yaml:"permission"`
	Ifdef      string `yaml:"ifdef"`
	P          *MenuDefinition
}

// Show checks permissions to see if we can display the menu item.
func (mi *MenuItem) Show(ctxt AmContext) bool {
	if mi.Permission == "" {
		return true
	}
	u := ctxt.CurrentUser()
	var eperm uint16
	switch mi.P.PermSet {
	case "user":
		eperm = u.BaseLevel
	case "community":
		eperm = ctxt.EffectiveLevel()
	default:
		eperm = database.AmRole("NotInList").Level()
	}
	if util.IsNumeric(mi.Permission) {
		v, _ := strconv.Atoi(mi.Permission)
		return uint16(v) <= eperm
	}
	switch mi.P.PermSet {
	case "user":
		return database.AmTestPermission(mi.Permission, eperm)
	case "community":
		return ctxt.CurrentCommunity().TestPermission(mi.Permission, eperm)
	default:
		return false
	}
}

// MenuDefinition represents a full menu definition.
type MenuDefinition struct {
	ID       string     `yaml:"id"`
	Title    string     `yaml:"title"`
	Subtitle string     `yaml:"subtitle"`
	PermSet  string     `yaml:"permSet"`
	Warning  string     `yaml:"warning"`
	Items    []MenuItem `yaml:"items"`
	Tag      string
}

// FilterCommunity creates a copy of this menu filtered to the specified community.
func (mdef *MenuDefinition) FilterCommunity(comm *database.Community) *MenuDefinition {
	newmd := MenuDefinition{
		ID:       mdef.ID,
		Title:    mdef.Title,
		Subtitle: strings.ReplaceAll(mdef.Subtitle, "[CNAME]", comm.Name),
		PermSet:  mdef.PermSet,
		Warning:  mdef.Warning,
		Items:    make([]MenuItem, len(mdef.Items)),
		Tag:      mdef.Tag,
	}
	for i, it := range mdef.Items {
		newmd.Items[i].Text = it.Text
		newmd.Items[i].Link = strings.ReplaceAll(it.Link, "[CID]", comm.Alias)
		newmd.Items[i].Disabled = it.Disabled
		newmd.Items[i].Hazard = it.Hazard
		newmd.Items[i].Permission = it.Permission
		newmd.Items[i].Ifdef = it.Ifdef
		newmd.Items[i].P = &newmd
	}
	return &newmd
}

// FilterConference creates a copy of this menu filtered to the specified community and conference.
func (mdef *MenuDefinition) FilterConference(comm *database.Community, confAlias string) *MenuDefinition {
	newmd := MenuDefinition{
		ID:       mdef.ID,
		Title:    mdef.Title,
		Subtitle: strings.ReplaceAll(mdef.Subtitle, "[CNAME]", comm.Name),
		PermSet:  mdef.PermSet,
		Warning:  mdef.Warning,
		Items:    make([]MenuItem, len(mdef.Items)),
		Tag:      mdef.Tag,
	}
	for i, it := range mdef.Items {
		newmd.Items[i].Text = it.Text
		s1 := strings.ReplaceAll(it.Link, "[CID]", comm.Alias)
		newmd.Items[i].Link = strings.ReplaceAll(s1, "[CONFID]", confAlias)
		newmd.Items[i].Disabled = it.Disabled
		newmd.Items[i].Hazard = it.Hazard
		newmd.Items[i].Permission = it.Permission
		newmd.Items[i].Ifdef = it.Ifdef
		newmd.Items[i].P = &newmd
	}
	return &newmd
}

// MenuDefs represents the set of all menu definitions.
type MenuDefs struct {
	D     []MenuDefinition `yaml:"menudefs"`
	table map[string]*MenuDefinition
}

//go:embed menudefs.yaml
var initMenuData []byte

// menuDefinitions gives the menu definitions.
var menuDefinitions MenuDefs

// Cache of community menus.
var menuCache *lru.Cache

// Mutex controlling access to the cache.
var menuCacheMutex sync.Mutex

// init loads the menu definitions.
func init() {
	if err := yaml.Unmarshal(initMenuData, &menuDefinitions); err != nil {
		panic(err) // can't happen
	}
	menuDefinitions.table = make(map[string]*MenuDefinition)
	for i, d := range menuDefinitions.D {
		menuDefinitions.table[d.ID] = &(menuDefinitions.D[i])
		for j := range menuDefinitions.D[i].Items {
			menuDefinitions.D[i].Items[j].P = &(menuDefinitions.D[i])
		}
		menuDefinitions.D[i].Tag = ""
	}
}

// setupMenus sets up the menu cache and the external menus.
func setupMenus() {
	var err error
	if menuCache, err = lru.New(config.GlobalConfig.Tuning.Caches.Menus); err != nil {
		panic(err)
	}
	mfile := config.GlobalConfig.ExPath(config.GlobalConfig.Resources.ExternalMenuDefinitions)
	if mfile != "" {
		b, err := os.ReadFile(mfile)
		if err == nil {
			md := new(MenuDefs)
			err = yaml.Unmarshal(b, md)
			if err == nil {
				for i, d := range md.D {
					menuDefinitions.table[d.ID] = &(md.D[i])
					for j := range md.D[i].Items {
						md.D[i].Items[j].P = &(md.D[i])
					}
				}
			} else {
				log.Errorf("cannot parse external menu definition file %s, ignored (%v)", mfile, err)
			}
		} else {
			log.Errorf("cannot read external menu definition file %s, ignored (%v)", mfile, err)
		}
	}
}

// AmMenu returns a menu definition.
func AmMenu(name string) *MenuDefinition {
	return menuDefinitions.table[name]
}

/* AmBuildCommunityMenu buids a community menu for the specified community.
 * Parameters:
 *     ctx - Standard Go context value.
 *     comm - The community to build the menu for.
 * Returns:
 *     The new menu definition.
 *     Standard Go error status.
 */
func AmBuildCommunityMenu(ctx context.Context, comm *database.Community) (*MenuDefinition, error) {
	menuCacheMutex.Lock()
	defer menuCacheMutex.Unlock()
	m, ok := menuCache.Get(comm.Id)
	if ok {
		return m.(*MenuDefinition), nil
	}
	sdef, err := database.AmGetCommunityServices(ctx, comm.Id)
	if err != nil {
		return nil, err
	}
	slices.SortFunc(sdef, func(a, b *database.ServiceDef) int {
		return a.LinkSequence - b.LinkSequence
	})
	mia := make([]MenuItem, len(sdef))
	md := MenuDefinition{
		ID:      "community",
		Title:   comm.Name,
		PermSet: "community",
		Tag:     "community",
		Items:   mia,
	}
	for i, sd := range sdef {
		mia[i].Text = sd.Title
		mia[i].Link = strings.ReplaceAll(sd.Link, "[CID]", comm.Alias)
		mia[i].Disabled = false
		mia[i].P = &md
		if sd.RequirePermission == "" {
			if sd.RequireRole == "" {
				mia[i].Permission = ""
			} else {
				mia[i].Permission = fmt.Sprintf("%d", database.AmRole(sd.RequireRole).Level())
			}
		} else if sd.RequireRole == "" {
			mia[i].Permission = sd.RequirePermission
		} else {
			v1 := comm.PermissionLevel(sd.RequirePermission)
			v2 := database.AmRole(sd.RequireRole).Level()
			if v2 > v1 {
				v1 = v2
			}
			mia[i].Permission = fmt.Sprintf("%d", v1)
		}
	}
	menuCache.Add(comm.Id, &md)
	return &md, nil
}

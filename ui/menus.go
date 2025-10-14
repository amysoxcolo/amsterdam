/*
 * Amsterdam Web Communities System
 * Copyright (c) 2025 Erbosoft Metaverse Design Solutions, All Rights Reserved
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */
// Package ui holds the support for the Amsterdam user interface, wrapping Echo and Jet templates.
package ui

import (
	_ "embed"

	"git.erbosoft.com/amy/amsterdam/database"
	"gopkg.in/yaml.v3"
)

// MenuItem represents an itrem within a menu definition.
type MenuItem struct {
	Text       string `yaml:"text"`
	Link       string `yaml:"link"`
	Disabled   bool   `yaml:"disabled"`
	Permission string `yaml:"permission"`
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
	default:
		eperm = database.AmRole("NotInList").Level()
	}
	return database.AmTestPermission(mi.Permission, eperm)
}

// MenuDefinition represents a full menu definition.
type MenuDefinition struct {
	ID      string     `yaml:"id"`
	Title   string     `yaml:"title"`
	PermSet string     `yaml:"permSet"`
	Warning string     `yaml:"warning"`
	Items   []MenuItem `yaml:"items"`
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
	}
}

// AmMenu returns a menu definition.
func AmMenu(name string) *MenuDefinition {
	return menuDefinitions.table[name]
}

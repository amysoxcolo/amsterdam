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

// AmLeftMenuItem represents an item on a left menu.
type AmLeftMenuItem struct {
	Text string
	Link string
}

// AmLeftMenu represents a left menu.
type AmLeftMenu struct {
	Title string
	Items []AmLeftMenuItem
}

// leftMenusRepo holds the possible left menus.
var leftMenusRepo = map[string]AmLeftMenu{}

// SetupLeftMenus parses the left menus into internal data structures.
func SetupLeftMenus() {
	// "Front Page" menu (not community specific)
	menu := AmLeftMenu{Title: "Front Page", Items: make([]AmLeftMenuItem, 2)}
	menu.Items[0] = AmLeftMenuItem{Text: "Calendar", Link: ""}
	menu.Items[1] = AmLeftMenuItem{Text: "Chat", Link: ""}
	leftMenusRepo["fp"] = menu

	// "About" menu (global)
	menu = AmLeftMenu{Title: "About This Site", Items: make([]AmLeftMenuItem, 2)}
	menu.Items[0] = AmLeftMenuItem{Text: "Documentation", Link: ""}
	menu.Items[1] = AmLeftMenuItem{Text: "About Amsterdam", Link: "/about"}
	leftMenusRepo["about"] = menu
}

/* augmentWithLeftMenus adds the left menus to the Amsterdam context.
 * Parameters:
 *     ctxt - The context to add the menus to.
 */
func augmentWithLeftMenus(ctxt AmContext) {
	mlist := make([]AmLeftMenu, 0, len(leftMenusRepo))
	// TODO: check for "in community" status and select menu
	mlist = append(mlist, leftMenusRepo["fp"])
	mlist = append(mlist, leftMenusRepo["about"])
	ctxt.VarMap().Set("amsterdam_leftMenus", mlist)
}

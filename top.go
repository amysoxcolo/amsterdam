/*
 * Amsterdam Web Communities System
 * Copyright (c) 2025 Erbosoft Metaverse Design Solutions, All Rights Reserved
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */
// Package main contains the high-level Amsterdam logic.
package main

import (
	"fmt"

	"git.erbosoft.com/amy/amsterdam/database"
	"git.erbosoft.com/amy/amsterdam/ui"
)

// RenderedSideboxItem is an item for display inside a rendered sidebox.
type RenderedSideboxItem struct {
	Text  string
	Link  *string
	Flags map[string]bool
}

// LinkX dereferences the Link pointer safely.
func (item *RenderedSideboxItem) LinkX() string {
	if item.Link == nil {
		return ""
	}
	return *item.Link
}

// RenderedSidebox is the data for a single rendered sidebox.
type RenderedSidebox struct {
	TemplateName string
	Title        string
	Subtext      *string
	Items        []RenderedSideboxItem
}

/* buildCommunitiesSidebox creates the data for the "My/Featured Communities" sidebox.
 * Parameters:
 *     uid - UID of the user rendering the page.
 *     out - The RenderedSidebox to be built.
 *     in - The sidebox data from the database.
 * Returns:
 *     Standard Go error status.
 */
func buildCommunitiesSidebox(uid int32, out *RenderedSidebox, in *database.Sidebox) error {
	var err error
	var anon bool
	anon, err = database.AmIsUserAnon(uid)
	if err == nil {
		if anon {
			out.Title = "Featured Communities"
		} else {
			out.Title = "Your Communities"
		}
		var l []*database.Community
		l, err = database.AmGetCommunitiesForUser(uid)
		if err == nil {
			out.Items = make([]RenderedSideboxItem, len(l))
			for i, c := range l {
				out.Items[i].Text = c.Name
				lk := fmt.Sprintf("/TODO/community/%s", c.Alias)
				out.Items[i].Link = &lk
				out.Items[i].Flags = make(map[string]bool)
				var level uint16
				level, err = database.AmGetCommunityAccessLevel(uid, c.Id)
				if err == nil && database.AmTestPermission("Community.ShowAdmin", level) {
					out.Items[i].Flags["admin"] = true
				}
			}
			out.TemplateName = "sb_ftrcomm.jet"
		}
	}
	_ = in
	return err
}

/* buildFeaturedConferences creates the data for the "Featured Conferences" sidebox.
 * Parameters:
 *     uid - UID of the user rendering the page.
 *     out - The RenderedSidebox to be built.
 *     in - The sidebox data from the database.
 * Returns:
 *     Standard Go error status.
 */
func buildFeaturedConferences(uid int32, out *RenderedSidebox, in *database.Sidebox) error {
	out.TemplateName = "sb_ftrconf.jet"
	return nil
}

/* buildUsersOnline creates the data for the "Users Online" sidebox.
 * Parameters:
 *     uid - UID of the user rendering the page.
 *     out - The RenderedSidebox to be built.
 *     in - The sidebox data from the database.
 * Returns:
 *     Standard Go error status.
 */
func buildUsersOnline(uid int32, out *RenderedSidebox, in *database.Sidebox) error {
	out.Title = "Users Online"
	out.TemplateName = "sb_online.jet"
	anons, users, maxUsers := ui.AmSessions()
	cap := len(users) + 1
	if anons > 0 {
		cap++
	}
	out.Items = make([]RenderedSideboxItem, cap)
	out.Items[0].Text = fmt.Sprintf("%d total (max %d)", len(users)+anons, maxUsers)
	out.Items[0].Flags = make(map[string]bool)
	out.Items[0].Flags["nobullet"] = true
	out.Items[0].Flags["bold"] = true
	b := 1
	if anons > 0 {
		out.Items[1].Text = fmt.Sprintf("Not logged in (%d)", anons)
		out.Items[1].Flags = make(map[string]bool)
		b++
	}
	for i, n := range users {
		out.Items[b+i].Text = n
		lk := fmt.Sprintf("/TODO/user/%s", n)
		out.Items[b+i].Link = &lk
		out.Items[b+i].Flags = make(map[string]bool)
		out.Items[b+i].Flags["bold"] = true
	}
	_ = uid
	_ = in
	return nil
}

/* buildRenderedSidebox creates a RenderedSidebox for the data in the database.
 * Parameters:
 *     uid - UID of the user rendering the page.
 *     out - The RenderedSidebox to be built.
 *     in - The sidebox data from the database.
 * Returns:
 *     Standard Go error status.
 */
func buildRenderedSidebox(uid int32, out *RenderedSidebox, in *database.Sidebox) error {
	switch in.Boxid {
	case 1:
		return buildCommunitiesSidebox(uid, out, in)
	case 2:
		return buildFeaturedConferences(uid, out, in)
	case 3:
		return buildUsersOnline(uid, out, in)
	default:
		return fmt.Errorf("unknown sidebox boxid: %d", in.Boxid)
	}
}

/* TopPage renders the "top level" Amsterdam page (the "home page").
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func TopPage(ctxt ui.AmContext) (string, any, error) {
	// Set the page title.
	ctxt.VarMap().Set("amsterdam_pageTitle", "My Front Page")

	// Retrieve the sideboxes and create the data to be presented.
	uid := ctxt.CurrentUserId()
	sboxes, err := database.AmGetSideboxes(uid)
	if err != nil {
		return "string", "Unable to retrieve sideboxes", err
	}

	rc := make([]RenderedSidebox, len(sboxes))
	for i, sb := range sboxes {
		err = buildRenderedSidebox(uid, &(rc[i]), sb)
		if err != nil {
			return "string", "Unable to render sideboxes", err
		}
	}
	ctxt.VarMap().Set("sideboxes", rc)

	// Final data set.
	ctxt.VarMap().Set("amsterdam_genRefresh", true)
	return "framed_template", "top.jet", nil
}

/* AboutPage renders the "About Amsterdam" page.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func AboutPage(ctxt ui.AmContext) (string, any, error) {
	// Set the page title.
	ctxt.VarMap().Set("amsterdam_pageTitle", "About Amsterdam")
	return "framed_template", "about.jet", nil
}

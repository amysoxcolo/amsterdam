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
	Flags []string
}

// RenderedSidebox is the data for a single rendered sidebox.
type RenderedSidebox struct {
	TemplateName string
	Title        string
	Subtext      *string
	Items        []RenderedSideboxItem
}

/* buildFeaturedCommunities creates the data for the "Featured Communities" sidebox.
 * Parameters:
 *     uid - UID of the user rendering the page.
 *     out - The RenderedSidebox to be built.
 *     in - The sidebox data from the database.
 * Returns:
 *     Standard Go error status.
 */
func buildFeaturedCommunities(uid int32, out *RenderedSidebox, in *database.Sidebox) error {
	out.TemplateName = "sb_ftrcomm.jet"
	return nil
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
	out.TemplateName = "sb_online.jet"
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
		return buildFeaturedCommunities(uid, out, in)
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
	uid := ctxt.Session().Values["user_id"].(int32)
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

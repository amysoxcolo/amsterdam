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

type RenderedSideboxItem struct {
	Text  string
	Link  *string
	Flags []string
}

type RenderedSidebox struct {
	TemplateName string
	Title        string
	Subtext      *string
	Items        []RenderedSideboxItem
}

func buildFeaturedCommunities(uid int32, out *RenderedSidebox, in *database.Sidebox) error {
	out.TemplateName = "sb_ftrcomm.jet"
	return nil
}

func buildFeaturedConferences(uid int32, out *RenderedSidebox, in *database.Sidebox) error {
	out.TemplateName = "sb_ftrconf.jet"
	return nil
}

func buildUsersOnline(uid int32, out *RenderedSidebox, in *database.Sidebox) error {
	out.TemplateName = "sb_online.jet"
	return nil
}

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

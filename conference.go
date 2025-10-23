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
	"errors"
	"net/http"

	"git.erbosoft.com/amy/amsterdam/database"
	"git.erbosoft.com/amy/amsterdam/ui"
)

// conferencesPrequel consolidates some of the basic conference checks into one function.
func conferencesPrequel(ctxt ui.AmContext) (string, any, error) {
	err := ctxt.SetCommunityContext(ctxt.URLParam("cid"))
	if err != nil {
		ctxt.SetRC(http.StatusNotFound)
		return ui.ErrorPage(ctxt, err)
	}
	comm := ctxt.CurrentCommunity()
	b, err := database.AmTestService(comm, "Conference")
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	if !b {
		ctxt.SetRC(http.StatusNotFound)
		return ui.ErrorPage(ctxt, errors.New("this community does not use conferencing services"))
	}
	if comm.MembersOnly && !ctxt.IsMember() {
		ctxt.SetRC(http.StatusForbidden)
		return ui.ErrorPage(ctxt, errors.New("you are not a member of this community"))
	}
	if !comm.TestPermission("Community.Read", ctxt.EffectiveLevel()) {
		ctxt.SetRC(http.StatusForbidden)
		return ui.ErrorPage(ctxt, errors.New("you are not authorized access to conferences"))
	}
	return "", nil, nil
}

/* Conferences displayes the list of conferences in a community.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func Conferences(ctxt ui.AmContext) (string, any, error) {
	cmd, arg, err := conferencesPrequel(ctxt)
	if cmd != "" {
		return cmd, arg, err
	}
	comm := ctxt.CurrentCommunity()
	ctxt.VarMap().Set("commName", comm.Name)
	ctxt.VarMap().Set("commAlias", comm.Alias)
	ctxt.VarMap().Set("amsterdam_pageTitle", "Conference Listing: "+comm.Name)
	clist, err := database.AmGetCommunityConferences(comm.Id,
		comm.TestPermission("Community.ShowHiddenObjects", ctxt.EffectiveLevel()))
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	ctxt.VarMap().Set("conferences", clist)
	return "framed_template", "conflist.jet", err
}

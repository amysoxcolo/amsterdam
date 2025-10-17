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

/* CommunityAdminMenu renders the community administration menu.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func CommunityAdminMenu(ctxt ui.AmContext) (string, any, error) {
	err := ctxt.SetCommunityContext(ctxt.URLParam("cid"))
	if err != nil {
		ctxt.SetRC(http.StatusNotFound)
		return ui.ErrorPage(ctxt, err)
	}
	comm := ctxt.CurrentCommunity()
	if !comm.TestPermission("Community.ShowAdmin", ctxt.EffectiveLevel()) {
		ctxt.SetRC(http.StatusForbidden)
		return ui.ErrorPage(ctxt, errors.New("you are not permitted to access this page"))
	}
	menu := ui.AmMenu("communityadmin")
	defs := make(map[string]bool)
	if !ctxt.GlobalFlags().Get(database.GlobalFlagNoCategories) {
		defs["USECAT"] = true
	}
	ctxt.SetLeftMenu("community")
	ctxt.VarMap().Set("menu", menu.FilterCommunity(comm))
	ctxt.VarMap().Set("defs", defs)
	ctxt.VarMap().Set("amsterdam_pageTitle", menu.Title+" - "+comm.Name)
	return "framed_template", "menu.jet", nil
}

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

/* SysAdminMenu renders the system administration menu.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func SysAdminMenu(ctxt ui.AmContext) (string, any, error) {
	u := ctxt.CurrentUser()
	if !database.AmTestPermission("Global.SysAdminAccess", u.BaseLevel) {
		ctxt.SetRC(http.StatusForbidden)
		return ui.ErrorPage(ctxt, errors.New("you are not authorized access to this page"))
	}
	menu := ui.AmMenu("sysadmin")
	ctxt.VarMap().Set("menu", menu)
	ctxt.VarMap().Set("amsterdam_pageTitle", menu.Title)
	return "framed_template", "menu.jet", nil
}

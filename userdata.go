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

	"git.erbosoft.com/amy/amsterdam/ui"
)

/* EditProfileForm renders the Amsterdam profile editing form.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func EditProfileForm(ctxt ui.AmContext) (string, any, error) {
	// Get target URI.
	target := ctxt.Parameter("tgt")
	if target == "" {
		target = "/"
	}
	u := ctxt.CurrentUser()
	if u.IsAnon {
		return ui.ErrorPage(ctxt, errors.New("you are not logged in"))
	}
	dlg, err := ui.AmLoadDialog("profile")
	if err == nil {
		dlg.Field("tgt").Value = target
		// TODO: load fields from current user
		return dlg.Render(ctxt)
	}
	return ui.ErrorPage(ctxt, err)
}

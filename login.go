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

import "git.erbosoft.com/amy/amsterdam/ui"

/* LoginForm renders the Amsterdam login form.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func LoginForm(ctxt ui.AmContext) (string, any, error) {
	dlg, err := ui.AmLoadDialog("login")
	if err == nil {
		ctxt.VarMap().Set("amsterdam_pageTitle", "Log In")
		return dlg.Render(ctxt)
	}
	return ui.ErrorPage(ctxt, err)
}

/* NewAccountUserAgreement renders the Amsterdam user agreement for new accounts.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func NewAccountUserAgreement(ctxt ui.AmContext) (string, any, error) {
	ctxt.VarMap().Set("amsterdam_pageTitle", "New Account User Agreement")
	return "framed_template", "agreement.jet", nil
}

/* NewAccountUserAgreement renders the Amsterdam account creation form.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func NewAccountForm(ctxt ui.AmContext) (string, any, error) {
	dlg, err := ui.AmLoadDialog("newaccount")
	if err == nil {
		dlg.Field("country").Value = "XX"
		ctxt.VarMap().Set("amsterdam_pageTitle", "Create New Account")
		return dlg.Render(ctxt)
	}
	return ui.ErrorPage(ctxt, err)
}

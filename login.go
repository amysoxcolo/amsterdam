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
	"fmt"

	"git.erbosoft.com/amy/amsterdam/database"
	"git.erbosoft.com/amy/amsterdam/email"
	"git.erbosoft.com/amy/amsterdam/ui"
)

/* LoginForm renders the Amsterdam login form.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func LoginForm(ctxt ui.AmContext) (string, any, error) {
	// Get target URI.
	target := ctxt.Parameter("tgt")
	if target == "" {
		target = "/"
	}

	// If user is already logged in, this is a no-op.
	if !ctxt.CurrentUser().IsAnon {
		return "redirect", target, nil
	}

	dlg, err := ui.AmLoadDialog("login")
	if err == nil {
		dlg.Field("tgt").Value = target
		return dlg.Render(ctxt)
	}
	return ui.ErrorPage(ctxt, err)
}

/* Login handles logging in to Amsterdam.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func Login(ctxt ui.AmContext) (string, any, error) {
	dlg, err := ui.AmLoadDialog("login")
	if err == nil {
		dlg.LoadFromForm(ctxt)
		target := dlg.Field("tgt").Value
		if target == "" {
			target = "/"
		}
		// If user is already logged in, this is a no-op.
		if !ctxt.CurrentUser().IsAnon {
			return "redirect", target, nil
		}

		action := dlg.WhichButton(ctxt)
		if action == "cancel" { // Cancel button pressed
			return "redirect", target, nil
		}
		if action == "remind" { // Password Reminder button pressed
			user, uerr := database.AmGetUserByName(dlg.Field("user").Value)
			if uerr == nil {
				var ci *database.ContactInfo
				ci, uerr = user.ContactInfo()
				if uerr == nil {
					if ci != nil && ci.Email != nil && *ci.Email != "" {
						msg := email.AmNewEmailMessage(ctxt.CurrentUserId(), ctxt.RemoteIP())
						msg.AddTo(*ci.Email, "")
						msg.SetTemplate("pass_remind.jet")
						msg.AddVariable("username", user.Username)
						msg.AddVariable("reminder", user.PassReminder)
						msg.AddVariable("change_uid", user.Uid)
						msg.AddVariable("change_auth", "TODO") // TODO: add change auth link
						msg.Send()
					} else {
						uerr = errors.New("cannot find email address")
					}
				}
			}

			dlg.Field("pass").Value = ""
			if uerr == nil {
				return dlg.RenderInfo(ctxt, "Password reminder has been sent to your E-mail address.")
			} else {
				return dlg.RenderError(ctxt, uerr.Error())
			}
		}
		if action == "login" { // Login button pressed
			// authenticate the user
			user, uerr := database.AmAuthenticateUser(dlg.Field("user").Value, dlg.Field("pass").Value, ctxt.RemoteIP())
			if uerr != nil {
				dlg.Field("pass").Value = ""
				return dlg.RenderError(ctxt, uerr.Error())
			}
			ctxt.ReplaceUser(user)
			if dlg.Field("saveme").IsChecked() {
				// TODO: cookie set
			}
			// TODO: bounce to E-mail verify if we can do so
			return "redirect", target, nil
		}
		dlg.Field("pass").Value = ""
		return dlg.RenderError(ctxt, "No known button click on POST to login function.")
	}
	return ui.ErrorPage(ctxt, err)
}

/* Logout handles logging out from Amsterdam.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func Logout(ctxt ui.AmContext) (string, any, error) {
	// Get target URI.
	target := ctxt.Parameter("tgt")
	if target == "" {
		target = "/"
	}

	if !ctxt.CurrentUser().IsAnon {
		// TODO: erase login cookie
		ctxt.ClearSession()
	}
	return "redirect", target, nil
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
	// Get target URI.
	target := ctxt.Parameter("tgt")
	if target == "" {
		target = "/"
	}

	// If user is already logged in, this is an error.
	if !ctxt.CurrentUser().IsAnon {
		return ui.ErrorPage(ctxt, fmt.Errorf("you cannot create a new account while logged in on an existing one. You must log out first"))
	}

	ctxt.VarMap().Set("target", target)
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
	// Get target URI.
	target := ctxt.Parameter("tgt")
	if target == "" {
		target = "/"
	}

	// If user is already logged in, this is an error.
	if !ctxt.CurrentUser().IsAnon {
		return ui.ErrorPage(ctxt, fmt.Errorf("you cannot create a new account while logged in on an existing one. You must log out first"))
	}

	dlg, err := ui.AmLoadDialog("newaccount")
	if err == nil {
		dlg.Field("tgt").Value = target
		dlg.Field("country").Value = "XX"
		return dlg.Render(ctxt)
	}
	return ui.ErrorPage(ctxt, err)
}

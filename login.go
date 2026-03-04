/*
 * Amsterdam Web Communities System
 * Copyright (c) 2025-2026 Erbosoft Metaverse Design Solutions, All Rights Reserved
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */
// Package main contains the high-level Amsterdam logic.
package main

import (
	"errors"
	"net/url"
	"time"

	"git.erbosoft.com/amy/amsterdam/database"
	"git.erbosoft.com/amy/amsterdam/email"
	"git.erbosoft.com/amy/amsterdam/ui"
	"git.erbosoft.com/amy/amsterdam/util"
	log "github.com/sirupsen/logrus"
)

// ENOACCOUNT is an error thrown if you have to log out before creating a new account.
var ENOACCOUNT error = errors.New("you cannot create a new account while logged in on an existing one. You must log out first")

/* LoginForm renders the Amsterdam login form.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func LoginForm(ctxt ui.AmContext) (string, any) {
	// Get target URI.
	target := ctxt.Parameter("tgt")
	if target == "" {
		v := ctxt.GetSession("lastKnownGood")
		if v != nil {
			target = v.(string)
		}
	}
	if target == "" {
		target = "/"
	}

	// If user is already logged in, this is a no-op.
	if !ctxt.CurrentUser().IsAnon {
		return "redirect", target
	}

	dlg, err := ui.AmLoadDialog("login")
	if err == nil {
		dlg.Field("tgt").Value = target
		return dlg.Render(ctxt)
	}
	return "error", err
}

/* Login handles logging in to Amsterdam.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func Login(ctxt ui.AmContext) (string, any) {
	dlg, err := ui.AmLoadDialog("login")
	if err == nil {
		dlg.LoadFromForm(ctxt)
		target := dlg.Field("tgt").Value
		if target == "" {
			v := ctxt.GetSession("lastKnownGood")
			if v != nil {
				target = v.(string)
			}
		}
		if target == "" {
			target = "/"
		}
		// If user is already logged in, this is a no-op.
		if !ctxt.CurrentUser().IsAnon {
			return "redirect", target
		}

		action := dlg.WhichButton(ctxt)
		if action == "cancel" { // Cancel button pressed
			return "redirect", target
		}
		username := dlg.Field("user").Value // since the dialog won't check this for us
		if len(username) == 0 {
			return dlg.RenderError(ctxt, "User name not specified.")
		}
		if action == "remind" { // Password Reminder button pressed
			user, uerr := database.AmGetUserByName(ctxt.Ctx(), username, nil)
			if uerr == nil {
				var ci *database.ContactInfo
				ci, uerr = user.ContactInfo(ctxt.Ctx())
				if uerr == nil {
					if ci != nil && ci.Email != nil && *ci.Email != "" {
						pchange := database.AmNewPasswordChangeRequest(user.Uid, user.Username, *ci.Email)
						msg := email.AmNewEmailMessage(ctxt.CurrentUserId(), ctxt.RemoteIP())
						msg.AddTo(*ci.Email, "")
						msg.SetTemplate("pass_remind.jet")
						msg.AddVariable("username", user.Username)
						msg.AddVariable("reminder", user.PassReminder)
						msg.AddVariable("change_uid", user.Uid)
						msg.AddVariable("change_auth", pchange.Authentication)
						msg.Send()
					} else {
						uerr = errors.New("cannot find email address")
					}
				}
			}

			if uerr == nil {
				return dlg.RenderInfo(ctxt, "Password reminder has been sent to your E-mail address.")
			} else {
				return dlg.RenderError(ctxt, uerr.Error())
			}
		}
		if action == "login" { // Login button pressed
			// authenticate the user
			user, uerr := database.AmAuthenticateUser(ctxt.Ctx(), username, dlg.Field("pass").Value, ctxt.RemoteIP())
			if uerr != nil {
				return dlg.RenderError(ctxt, uerr.Error())
			}
			ctxt.ReplaceUser(user)
			if dlg.Field("saveme").IsChecked() {
				// create and save an authentication token
				authString, cerr := user.NewAuthToken(ctxt.Ctx())
				if cerr == nil {
					ctxt.SetLoginCookie(authString)

				} else {
					log.Errorf("unable to generate auth string for uid %d: %v", user.Uid, cerr)
				}
			}
			if user.VerifyEMail {
				return "redirect", target
			} else {
				return "redirect", "/verify?tgt=" + url.QueryEscape(target)
			}
		}
		return dlg.RenderError(ctxt, "No known button click on POST to login function.")
	}
	return "error", err
}

/* Logout handles logging out from Amsterdam.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func Logout(ctxt ui.AmContext) (string, any) {
	// Get target URI.
	target := ctxt.Parameter("tgt")
	if target == "" {
		target = "/"
	}

	if !ctxt.CurrentUser().IsAnon {
		ctxt.ClearLoginCookie()
		ctxt.ClearSession()
	}
	return "redirect", target
}

/* VerifyEmailForm renders the E-mail address verification form.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func VerifyEmailForm(ctxt ui.AmContext) (string, any) {
	// Get target URI.
	target := ctxt.Parameter("tgt")
	if target == "" {
		v := ctxt.GetSession("lastKnownGood")
		if v != nil {
			target = v.(string)
		}
	}
	if target == "" {
		target = "/"
	}

	// If user is not logged in, this is an error.
	user := ctxt.CurrentUser()
	if user.IsAnon {
		return "error", ELOGIN
	}

	// If user is already verified, this is a no-op.
	if user.VerifyEMail {
		return "redirect", target
	}

	dlg, err := ui.AmLoadDialog("verify_email")
	if err == nil {
		dlg.Field("tgt").Value = target
		return dlg.Render(ctxt)
	}
	return "error", err
}

// sendEmailConfirmationEmail sends the "E-mail confirmation number" E-mail message.
func sendEmailConfirmationEmail(user *database.User, ci *database.ContactInfo, remoteIP string) error {
	if ci != nil && ci.Email != nil && *ci.Email != "" {
		msg := email.AmNewEmailMessage(user.Uid, remoteIP)
		msg.AddTo(*ci.Email, "")
		msg.SetTemplate("email_confirm.jet")
		msg.AddVariable("username", user.Username)
		msg.AddVariable("confnum", user.EmailConfNum)
		msg.Send()
		return nil
	} else {
		return errors.New("cannot find email address")
	}
}

/* VerifyEmail handles E-mail address verification.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func VerifyEMail(ctxt ui.AmContext) (string, any) {
	// If user is not logged in, this is an error.
	user := ctxt.CurrentUser()
	if user.IsAnon {
		return "error", ELOGIN
	}

	dlg, err := ui.AmLoadDialog("verify_email")
	if err == nil {
		dlg.LoadFromForm(ctxt)
		target := dlg.Field("tgt").Value
		if target == "" {
			v := ctxt.GetSession("lastKnownGood")
			if v != nil {
				target = v.(string)
			}
		}
		if target == "" {
			target = "/"
		}

		// If user is already verified, this is a no-op.
		if user.VerifyEMail {
			return "redirect", target
		}

		action := dlg.WhichButton(ctxt)
		if action == "cancel" { // Cancel button pressed
			return "redirect", target
		}
		if action == "sendagain" {
			var ci *database.ContactInfo
			ci, err = user.ContactInfo(ctxt.Ctx())
			if err == nil {
				err = user.NewEmailConfirmationNumber(ctxt.Ctx())
				if err == nil {
					err = sendEmailConfirmationEmail(user, ci, ctxt.RemoteIP())
				}
			}
			if err == nil {
				return dlg.RenderInfo(ctxt, "Verification message has been sent to your E-mail address.")
			} else {
				return dlg.RenderError(ctxt, err.Error())
			}
		}
		if action == "ok" {
			err = dlg.Validate()
			if err == nil {
				cn, _ := dlg.Field("num").ValueInt()
				err = user.ConfirmEMailAddress(ctxt.Ctx(), int32(cn), ctxt.RemoteIP())
				if err == nil {
					return "redirect", target
				}
			}
			return dlg.RenderError(ctxt, err.Error())
		}
		return dlg.RenderError(ctxt, "No known button click on POST to verify function.")
	}
	return "error", err
}

/* NewAccountUserAgreement renders the Amsterdam user agreement for new accounts.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func NewAccountUserAgreement(ctxt ui.AmContext) (string, any) {
	// Get target URI.
	target := ctxt.Parameter("tgt")
	if target == "" {
		v := ctxt.GetSession("lastKnownGood")
		if v != nil {
			target = v.(string)
		}
	}
	if target == "" {
		target = "/"
	}

	// If user is already logged in, this is an error.
	if !ctxt.CurrentUser().IsAnon {
		return "error", ENOACCOUNT
	}

	// Load the user agreement from the resources.
	agreementTitle, agreementBody, err := ui.AmLoadHTMLResource("useragreement.html")
	if err != nil {
		return "error", err
	}

	ctxt.SetLeftMenu("top")
	ctxt.VarMap().Set("target", target)
	ctxt.VarMap().Set("agreementTitle", agreementTitle)
	ctxt.VarMap().Set("agreementBody", agreementBody)
	ctxt.SetScratch("frame_suppressLogin", true)
	ctxt.SetFrameTitle("New Account User Agreement")
	return "framed", "agreement.jet"
}

/* NewAccountUserAgreement renders the Amsterdam account creation form.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func NewAccountForm(ctxt ui.AmContext) (string, any) {
	// Get target URI.
	target := ctxt.Parameter("tgt")
	if target == "" {
		v := ctxt.GetSession("lastKnownGood")
		if v != nil {
			target = v.(string)
		}
	}
	if target == "" {
		target = "/"
	}

	// If user is already logged in, this is an error.
	if !ctxt.CurrentUser().IsAnon {
		return "error", ENOACCOUNT
	}

	dlg, err := ui.AmLoadDialog("newaccount")
	if err == nil {
		dlg.Field("tgt").Value = target
		dlg.Field("country").Value = "XX"
		return dlg.Render(ctxt)
	}
	return "error", err
}

/* NewAccount handles creating a new Amsterdam account.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func NewAccount(ctxt ui.AmContext) (string, any) {
	// If user is already logged in, this is an error.
	if !ctxt.CurrentUser().IsAnon {
		return "error", ENOACCOUNT
	}

	dlg, err := ui.AmLoadDialog("newaccount")
	if err == nil {
		dlg.LoadFromForm(ctxt)
		target := dlg.Field("tgt").Value
		if target == "" {
			v := ctxt.GetSession("lastKnownGood")
			if v != nil {
				target = v.(string)
			}
		}
		if target == "" {
			target = "/"
		}

		action := dlg.WhichButton(ctxt)
		if action == "cancel" { // Cancel button pressed
			return "redirect", target
		}
		if action == "create" {
			err = dlg.Validate()
			if err == nil {
				if dlg.Field("pass1").Value != dlg.Field("pass2").Value {
					return dlg.RenderError(ctxt, "The typed passwords do not match.")
				}
				var banned bool
				banned, err = database.AmIsEmailAddressBanned(ctxt.Ctx(), dlg.Field("email").Value)
				if err == nil {
					if banned {
						return dlg.RenderError(ctxt, "This E-mail address may not register a new account.")
					}
					// Create new user account
					var user *database.User
					user, err = database.AmCreateNewUser(ctxt.Ctx(), dlg.Field("user").Value, dlg.Field("pass1").Value,
						dlg.Field("remind").Value, dlg.Field("dob").AsDate(), ctxt.RemoteIP())
					if err == nil {
						// create and save contact info
						ci := database.AmNewUserContactInfo(user.Uid)
						ci.Prefix = dlg.Field("prefix").ValPtr()
						ci.GivenName = dlg.Field("first").ValPtr()
						mid := dlg.Field("mid").Value
						if mid == "" {
							mid = " "
						}
						ci.MiddleInit = &mid
						ci.FamilyName = dlg.Field("last").ValPtr()
						ci.Suffix = dlg.Field("suffix").ValPtr()
						ci.Locality = dlg.Field("loc").ValPtr()
						ci.Region = dlg.Field("reg").ValPtr()
						ci.PostalCode = dlg.Field("pcode").ValPtr()
						ci.Country = dlg.Field("country").ValPtr()
						ci.Email = dlg.Field("email").ValPtr()
						_, err = ci.Save(ctxt.Ctx(), user, ctxt.RemoteIP())
						if err == nil {
							err = user.SetContactID(ctxt.Ctx(), ci.ContactId)
						}
						if err == nil {
							err = sendEmailConfirmationEmail(user, ci, ctxt.RemoteIP())
						}
						if err == nil {
							// user is now logged in! redirect to E-mail verification
							ctxt.ReplaceUser(user)
							return "redirect", "/verify?tgt=" + url.QueryEscape(target)
						}
					}
				}
			}
			return dlg.RenderError(ctxt, err.Error())
		}
		return dlg.RenderError(ctxt, "No known button click on POST to new account.")
	}
	return "error", err
}

/* PasswordRecovery handles a click on a "password recovery" link to fix the user's password.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func PasswordRecovery(ctxt ui.AmContext) (string, any) {
	var emailaddy string
	uid, err := ctxt.URLParamInt("uid")
	if err == nil {
		auth, err := ctxt.URLParamInt("auth")
		if err == nil {
			pchange := database.AmGetPasswordChangeRequest(int32(uid))
			if pchange == nil {
				return "error", "password change request not found"
			}
			if auth != int(pchange.Authentication) {
				return "error", "invalid password change request"
			}
			if time.Now().Compare(pchange.Expires) > 0 {
				return "error", "password change request has expired"
			}
			emailaddy = pchange.Email
		}
	}

	if err == nil {
		user, err := database.AmGetUser(ctxt.Ctx(), int32(uid))
		if err == nil {
			newpass := util.GenerateRandomPassword()
			err = user.ChangePassword(ctxt.Ctx(), newpass, user, ctxt.RemoteIP())
			if err == nil {
				// send the password change message
				msg := email.AmNewEmailMessage(user.Uid, ctxt.RemoteIP())
				msg.AddTo(emailaddy, "")
				msg.SetTemplate("pass_change.jet")
				msg.AddVariable("username", user.Username)
				msg.AddVariable("password", newpass)
				msg.Send()
				ctxt.SetLeftMenu("top")
				ctxt.SetFrameTitle("Your Password Has Been Changed")
				return "framed", "password_changed.jet"
			}
		}
	}
	return "error", err
}

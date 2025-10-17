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
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"git.erbosoft.com/amy/amsterdam/database"
	"git.erbosoft.com/amy/amsterdam/email"
	"git.erbosoft.com/amy/amsterdam/ui"
	"git.erbosoft.com/amy/amsterdam/util"
	"github.com/biter777/countries"
	log "github.com/sirupsen/logrus"
)

// userPhotoURL returns the photo URL from the contact info, or a default.
func userPhotoURL(ci *database.ContactInfo) string {
	if ci.PhotoURL != nil && *ci.PhotoURL != "" {
		return *ci.PhotoURL
	}
	return "/img/builtin/no-user.png"
}

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
		ctxt.VarMap().Set("target", target)
		var ci *database.ContactInfo
		ci, err = u.ContactInfo()
		if err == nil {
			var prefs *database.UserPrefs
			prefs, err = u.Prefs()
			if err == nil {
				dlg.Field("remind").Value = u.PassReminder
				dlg.Field("prefix").SetVal(ci.Prefix)
				dlg.Field("first").SetVal(ci.GivenName)
				dlg.Field("mid").SetVal(ci.MiddleInit)
				dlg.Field("last").SetVal(ci.FamilyName)
				dlg.Field("suffix").SetVal(ci.Suffix)
				dlg.Field("company").SetVal(ci.Company)
				dlg.Field("addr1").SetVal(ci.Addr1)
				dlg.Field("addr2").SetVal(ci.Addr2)
				dlg.Field("pvt_addr").SetChecked(ci.PrivateAddr)
				dlg.Field("loc").SetVal(ci.Locality)
				dlg.Field("reg").SetVal(ci.Region)
				dlg.Field("pcode").SetVal(ci.PostalCode)
				dlg.Field("country").SetVal(ci.Country)
				dlg.Field("phone").SetVal(ci.Phone)
				dlg.Field("mobile").SetVal(ci.Mobile)
				dlg.Field("pvt_phone").SetChecked(ci.PrivatePhone)
				dlg.Field("fax").SetVal(ci.Fax)
				dlg.Field("pvt_fax").SetChecked(ci.PrivateFax)
				dlg.Field("email").SetVal(ci.Email)
				dlg.Field("pvt_email").SetChecked(ci.PrivateEmail)
				dlg.Field("url").SetVal(ci.URL)
				dlg.Field("dob").SetDate(u.DOB)
				dlg.Field("descr").SetVal(u.Description)
				dlg.Field("photo").Value = userPhotoURL(ci)
				dlg.Field("pic_in_post").SetChecked(u.FlagValue(database.UserFlagPicturesInPosts))
				dlg.Field("no_mass_mail").SetChecked(u.FlagValue(database.UserFlagMassMailOptOut))
				dlg.Field("locale").Value = prefs.ReadLocale()
				dlg.Field("tz").Value = prefs.TimeZoneID
				return dlg.Render(ctxt)
			}
		}
	}
	return ui.ErrorPage(ctxt, err)
}

/* EditProfile handles profile editing.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func EditProfile(ctxt ui.AmContext) (string, any, error) {
	u := ctxt.CurrentUser()
	if u.IsAnon {
		return ui.ErrorPage(ctxt, errors.New("you are not logged in"))
	}
	dlg, err := ui.AmLoadDialog("profile")
	if err == nil {
		dlg.LoadFromForm(ctxt)
		target := dlg.Field("tgt").Value
		if target == "" {
			target = "/"
		}
		ctxt.VarMap().Set("target", target)

		action := dlg.WhichButton(ctxt)
		if action == "cancel" { // Cancel button pressed
			return "redirect", target, nil
		}
		if action == "update" {
			var ci *database.ContactInfo
			ci, err = u.ContactInfo()
			if err == nil {
				var prefs *database.UserPrefs
				emailChange := false
				prefs, err = u.Prefs()
				if err == nil && !(dlg.Field("pass1").IsEmpty() && dlg.Field("pass2").IsEmpty()) {
					p1 := dlg.Field("pass1").Value
					if p1 == dlg.Field("pass2").Value {
						err = u.ChangePassword(p1, ctxt.RemoteIP())
					} else {
						err = errors.New("passwords do not match")
					}
				}
				if err == nil {
					nci := ci.Clone()
					nci.Prefix = dlg.Field("prefix").ValPtr()
					nci.GivenName = dlg.Field("first").ValPtr()
					nci.MiddleInit = dlg.Field("mid").ValPtr()
					nci.FamilyName = dlg.Field("last").ValPtr()
					nci.Suffix = dlg.Field("suffix").ValPtr()
					nci.Company = dlg.Field("company").ValPtr()
					nci.Addr1 = dlg.Field("addr1").ValPtr()
					nci.Addr2 = dlg.Field("addr2").ValPtr()
					nci.PrivateAddr = dlg.Field("pvt_addr").IsChecked()
					nci.Locality = dlg.Field("loc").ValPtr()
					nci.Region = dlg.Field("reg").ValPtr()
					nci.PostalCode = dlg.Field("pcode").ValPtr()
					nci.Country = dlg.Field("country").ValPtr()
					nci.Phone = dlg.Field("phone").ValPtr()
					nci.Mobile = dlg.Field("mobile").ValPtr()
					nci.PrivatePhone = dlg.Field("pvt_phone").IsChecked()
					nci.Fax = dlg.Field("fax").ValPtr()
					nci.PrivateFax = dlg.Field("pvt_fax").IsChecked()
					nci.Email = dlg.Field("email").ValPtr()
					nci.PrivateEmail = dlg.Field("pvt_email").IsChecked()
					nci.URL = dlg.Field("url").ValPtr()
					emailChange, err = nci.Save()
					ci = nci
				}
				if err == nil {
					nprefs := prefs.Clone()
					nprefs.WriteLocale(dlg.Field("locale").Value)
					nprefs.TimeZoneID = dlg.Field("tz").Value
					err = nprefs.Save(u)
				}
				if err == nil {
					var f *util.OptionSet
					f, err = u.Flags()
					if err == nil {
						nf := f.Clone()
						nf.Set(database.UserFlagPicturesInPosts, dlg.Field("pic_in_post").IsChecked())
						nf.Set(database.UserFlagMassMailOptOut, dlg.Field("no_mass_mail").IsChecked())
						err = u.SaveFlags(nf)
					}
				}
				if err == nil {
					err = u.SetProfileData(dlg.Field("remind").Value, dlg.Field("dob").AsDate(), dlg.Field("descr").ValPtr())
				}
				if err == nil {
					if emailChange {
						err = sendEmailConfirmationEmail(u, ci, ctxt.RemoteIP())
						if err == nil {
							return "redirect", "/verify?tgt=" + url.QueryEscape(target), nil
						}
					} else {
						return "redirect", target, nil
					}
				}
			}
		}
		return dlg.RenderError(ctxt, "No known button click on POST to profile.")
	}
	return ui.ErrorPage(ctxt, err)
}

/* ProfilePhotoForm renders the Amsterdam profile photo upload form.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func ProfilePhotoForm(ctxt ui.AmContext) (string, any, error) {
	// Get target URI.
	target := ctxt.Parameter("tgt")
	if target == "" {
		target = "/"
	}
	u := ctxt.CurrentUser()
	if u.IsAnon {
		return ui.ErrorPage(ctxt, errors.New("you are not logged in"))
	}
	ci, err := u.ContactInfo()
	if err == nil {
		ctxt.VarMap().Set("target", target)
		ctxt.VarMap().Set("photo_url", userPhotoURL(ci))
		ctxt.VarMap().Set("amsterdam_pageTitle", "Upload User Photo")
		ctxt.VarMap().Set("amsterdam_suppressLogin", true)
		return "framed_template", "photo_upload.jet", nil
	}
	return ui.ErrorPage(ctxt, err)
}

/* ProfilePhoto handles processing the uploaded user photo.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func ProfilePhoto(ctxt ui.AmContext) (string, any, error) {
	u := ctxt.CurrentUser()
	if u.IsAnon {
		return ui.ErrorPage(ctxt, errors.New("you are not logged in"))
	}
	ci, err := u.ContactInfo()
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	target := ctxt.FormField("tgt")
	if target == "" {
		target = "/"
	}
	if ctxt.FormFieldIsSet("cancel") {
		return "redirect", "/profile?tgt=" + url.QueryEscape(target), nil
	}
	if ctxt.FormFieldIsSet("upload") {
		file, err := ctxt.FormFile("thepic")
		if err == nil {
			var imageData []byte
			var mimeType string
			imageData, mimeType, err = ui.AmProcessUploadedImage(file, ui.UserPhotoWidth, ui.UserPhotoHeight,
				ui.UserPhotoMaxBytes)
			if err == nil {
				var img *database.ImageStore
				img, err = database.AmStoreImage(database.ImageTypeUserPhoto, u.Uid, mimeType, imageData)
				if err == nil {
					photourl := fmt.Sprintf("/img/store/%d", img.ImgId)
					ci.PhotoURL = &photourl
					_, err = ci.Save()
					if err == nil {
						return "redirect", "/profile?tgt=" + url.QueryEscape(target), nil
					}
				}
			}
		}
		ctxt.VarMap().Set("errorMessage", err.Error())
		ctxt.VarMap().Set("target", target)
		ctxt.VarMap().Set("photo_url", userPhotoURL(ci))
		ctxt.VarMap().Set("amsterdam_pageTitle", "Upload User Photo")
		ctxt.VarMap().Set("amsterdam_suppressLogin", true)
		return "framed_template", "photo_upload.jet", nil
	}
	if ctxt.FormFieldIsSet("remove") {
		purl := ci.PhotoURL
		happy := false
		if purl == nil || *purl == "" {
			// this is a no-op
			return "redirect", "/profile?tgt=" + url.QueryEscape(target), nil
		}
		if strings.HasPrefix(*purl, "/img/store/") {
			id, err := strconv.Atoi((*purl)[11:])
			if err != nil {
				return ui.ErrorPage(ctxt, err)
			}
			defer func() {
				if happy {
					go func() {
						err := database.AmDeleteImage(int32(id))
						if err != nil {
							log.Errorf("unable to delete image ID %d: %v", id, err)
						}
					}()
				}
			}()
		}
		ci.PhotoURL = nil
		_, err := ci.Save()
		if err != nil {
			return ui.ErrorPage(ctxt, err)
		}
		happy = true
		return "redirect", "/profile?tgt=" + url.QueryEscape(target), nil
	}
	return ui.ErrorPage(ctxt, errors.New("invalid button detected in photo upload"))
}

/* ShowProfile displays a user's profile.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func ShowProfile(ctxt ui.AmContext) (string, any, error) {
	me := ctxt.CurrentUser()
	prefs, err := me.Prefs()
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}

	// Gather the info on the current user.
	user, err := database.AmGetUserByName(ctxt.URLParam("uname"))
	if err != nil {
		ctxt.SetRC(http.StatusNotFound)
		return ui.ErrorPage(ctxt, err)
	}
	ci, err := user.ContactInfo()
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	var pvtAddr, pvtPhone, pvtFax, pvtEmail bool
	if database.AmTestPermission("Global.SeeHiddenContactInfo", me.BaseLevel) {
		pvtAddr = false
		pvtPhone = false
		pvtFax = false
		pvtEmail = false
	} else {
		pvtAddr = ci.PrivateAddr
		pvtPhone = ci.PrivatePhone
		pvtFax = ci.PrivateFax
		pvtEmail = ci.PrivateEmail
	}

	// Fill all the page variables for display.
	ctxt.VarMap().Set("uid", user.Uid)
	ctxt.VarMap().Set("username", user.Username)
	ctxt.VarMap().Set("photoURL", userPhotoURL(ci))
	tz := prefs.Location()
	loc := prefs.Localizer()
	ctxt.VarMap().Set("dateCreated", loc.Strftime("%x %X", user.Created.In(tz)))
	if user.LastAccess != nil {
		ctxt.VarMap().Set("dateLastLogin", loc.Strftime("%x %X", (*user.LastAccess).In(tz)))
	}
	if ci.LastUpdate != nil {
		ctxt.VarMap().Set("dateLastUpdate", loc.Strftime("%x %X", (*ci.LastUpdate).In(tz)))
	}
	var b strings.Builder
	if ci.Prefix != nil && *ci.Prefix != "" {
		b.WriteString(*ci.Prefix + " ")
	}
	if ci.GivenName != nil {
		b.WriteString(*ci.GivenName)
	}
	if ci.MiddleInit != nil && *ci.MiddleInit != "" && *ci.MiddleInit != " " {
		b.WriteString(" " + *ci.MiddleInit + ".")
	}
	if ci.FamilyName != nil {
		b.WriteString(" " + *ci.FamilyName)
	}
	if ci.Suffix != nil && *ci.Suffix != "" {
		b.WriteString(" " + *ci.Suffix)
	}
	ctxt.VarMap().Set("fullname", b.String())
	if user.Description != nil {
		ctxt.VarMap().Set("description", *user.Description)
	}
	if !pvtEmail && ci.Email != nil {
		ctxt.VarMap().Set("email", *ci.Email)
	}
	if ci.URL != nil && *ci.URL != "" {
		ctxt.VarMap().Set("user_url", *ci.URL)
	}
	if ci.Company != nil {
		ctxt.VarMap().Set("company", *ci.Company)
	}
	if !pvtAddr && ci.Addr1 != nil {
		ctxt.VarMap().Set("addr1", *ci.Addr1)
	}
	if !pvtAddr && ci.Addr2 != nil {
		ctxt.VarMap().Set("addr2", *ci.Addr2)
	}
	b.Reset()
	if ci.Locality != nil {
		b.WriteString(*ci.Locality)
		if ci.Region != nil {
			b.WriteString(", ")
		}
	}
	if ci.Region != nil {
		b.WriteString(*ci.Region)
	}
	if ci.PostalCode != nil {
		b.WriteString("  " + *ci.PostalCode)
	}
	ctxt.VarMap().Set("addrLast", b.String())
	if ci.Country != nil {
		country := countries.ByName(*ci.Country)
		ctxt.VarMap().Set("country", country.String())
	}
	if !pvtPhone && ci.Phone != nil {
		ctxt.VarMap().Set("phone", *ci.Phone)
	}
	if !pvtFax && ci.Fax != nil {
		ctxt.VarMap().Set("fax", *ci.Fax)
	}
	if !pvtPhone && ci.Mobile != nil {
		ctxt.VarMap().Set("mobile", *ci.Mobile)
	}
	ctxt.VarMap().Set("amsterdam_pageTitle", fmt.Sprintf("User Profile - %s", user.Username))
	return "framed_template", "profile.jet", nil
}

/* QuickEMail sends quick E-mail to a user.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func QuickEMail(ctxt ui.AmContext) (string, any, error) {
	me := ctxt.CurrentUser()
	if me.IsAnon {
		return ui.ErrorPage(ctxt, errors.New("you are not logged in"))
	}
	myCI, err := me.ContactInfo()
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	toUid, err := ctxt.FormFieldInt("to_uid")
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	user, err := database.AmGetUser(int32(toUid))
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	if user.IsAnon {
		return ui.ErrorPage(ctxt, errors.New("cannot send quick E-mail to anonymous user"))
	}
	ci, err := user.ContactInfo()
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	msg := email.AmNewEmailMessage(me.Uid, ctxt.RemoteIP())
	msg.AddTo(*ci.Email, user.Username)
	msg.AddHeader("X-Originally-From", fmt.Sprintf("%s <%s>", me.Username, *myCI.Email))
	msg.SetSubject(ctxt.FormField("subj"))
	msg.SetText(ctxt.FormField("pb"))
	msg.Send()
	return "redirect", "/user/" + user.Username, nil
}

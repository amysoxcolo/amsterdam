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
	"context"
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
	"github.com/labstack/echo/v4"
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
 */
func EditProfileForm(ctxt ui.AmContext) (string, any) {
	// Get target URI.
	target := ctxt.Parameter("tgt")
	if target == "" {
		target = "/"
	}
	u := ctxt.CurrentUser()
	if u.IsAnon {
		return "error", ELOGIN
	}
	dlg, err := ui.AmLoadDialog("profile")
	if err == nil {
		dlg.Field("tgt").Value = target
		ctxt.VarMap().Set("target", target)
		var ci *database.ContactInfo
		ci, err = u.ContactInfo(ctxt.Ctx())
		if err == nil {
			var prefs *database.UserPrefs
			prefs, err = u.Prefs(ctxt.Ctx())
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
				dlg.Field("pic_in_post").SetChecked(u.FlagValue(ctxt.Ctx(), database.UserFlagPicturesInPosts))
				dlg.Field("no_mass_mail").SetChecked(u.FlagValue(ctxt.Ctx(), database.UserFlagMassMailOptOut))
				dlg.Field("locale").Value = prefs.ReadLocale()
				dlg.Field("tz").Value = prefs.TimeZoneID
				return dlg.Render(ctxt)
			}
		}
	}
	return "error", err
}

/* EditProfile handles profile editing.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func EditProfile(ctxt ui.AmContext) (string, any) {
	u := ctxt.CurrentUser()
	if u.IsAnon {
		return "error", ELOGIN
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
			return "redirect", target
		}
		if action == "update" {
			err = dlg.Validate()
			if err != nil {
				return dlg.RenderError(ctxt, err.Error())
			}
			var ci *database.ContactInfo
			ci, err = u.ContactInfo(ctxt.Ctx())
			if err == nil {
				var prefs *database.UserPrefs
				emailChange := false
				prefs, err = u.Prefs(ctxt.Ctx())
				if err == nil && !(dlg.Field("pass1").IsEmpty() && dlg.Field("pass2").IsEmpty()) {
					p1 := dlg.Field("pass1").Value
					if p1 == dlg.Field("pass2").Value {
						err = u.ChangePassword(ctxt.Ctx(), p1, ctxt.RemoteIP())
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
					emailChange, err = nci.Save(ctxt.Ctx())
					ci = nci
				}
				if err == nil {
					nprefs := prefs.Clone()
					nprefs.WriteLocale(dlg.Field("locale").Value)
					nprefs.TimeZoneID = dlg.Field("tz").Value
					err = nprefs.Save(ctxt.Ctx(), u)
				}
				if err == nil {
					var f *util.OptionSet
					f, err = u.Flags(ctxt.Ctx())
					if err == nil {
						nf := f.Clone()
						nf.Set(database.UserFlagPicturesInPosts, dlg.Field("pic_in_post").IsChecked())
						nf.Set(database.UserFlagMassMailOptOut, dlg.Field("no_mass_mail").IsChecked())
						err = u.SaveFlags(ctxt.Ctx(), nf)
					}
				}
				if err == nil {
					err = u.SetProfileData(ctxt.Ctx(), dlg.Field("remind").Value, dlg.Field("dob").AsDate(), dlg.Field("descr").ValPtr())
				}
				if err == nil {
					if emailChange {
						err = sendEmailConfirmationEmail(u, ci, ctxt.RemoteIP())
						if err == nil {
							return "redirect", "/verify?tgt=" + url.QueryEscape(target)
						}
					} else {
						return "redirect", target
					}
				}
			}
		}
		return dlg.RenderError(ctxt, "No known button click on POST to profile.")
	}
	return "error", err
}

/* ProfilePhotoForm renders the Amsterdam profile photo upload form.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func ProfilePhotoForm(ctxt ui.AmContext) (string, any) {
	// Get target URI.
	target := ctxt.Parameter("tgt")
	if target == "" {
		target = "/"
	}
	u := ctxt.CurrentUser()
	if u.IsAnon {
		return "error", ELOGIN
	}
	ci, err := u.ContactInfo(ctxt.Ctx())
	if err == nil {
		ctxt.VarMap().Set("target", target)
		ctxt.VarMap().Set("photo_url", userPhotoURL(ci))
		ctxt.SetScratch("frame_suppressLogin", true)
		ctxt.SetFrameTitle("Upload User Photo")
		return "framed", "photo_upload.jet"
	}
	return "error", err
}

/* ProfilePhoto handles processing the uploaded user photo.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func ProfilePhoto(ctxt ui.AmContext) (string, any) {
	u := ctxt.CurrentUser()
	if u.IsAnon {
		return "error", ELOGIN
	}
	ci, err := u.ContactInfo(ctxt.Ctx())
	if err != nil {
		return "error", err
	}
	target := ctxt.FormField("tgt")
	if target == "" {
		target = "/"
	}
	if ctxt.FormFieldIsSet("cancel") {
		return "redirect", "/profile?tgt=" + url.QueryEscape(target)
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
				img, err = database.AmStoreImage(ctxt.Ctx(), database.ImageTypeUserPhoto, u.Uid, mimeType, imageData)
				if err == nil {
					photourl := fmt.Sprintf("/img/store/%d", img.ImgId)
					ci.PhotoURL = &photourl
					_, err = ci.Save(ctxt.Ctx())
					if err == nil {
						return "redirect", "/profile?tgt=" + url.QueryEscape(target)
					}
				}
			}
		}
		ctxt.VarMap().Set("errorMessage", err.Error())
		ctxt.VarMap().Set("target", target)
		ctxt.VarMap().Set("photo_url", userPhotoURL(ci))
		ctxt.SetScratch("frame_suppressLogin", true)
		ctxt.SetFrameTitle("Upload User Photo")
		return "framed", "photo_upload.jet"
	}
	if ctxt.FormFieldIsSet("remove") {
		purl := ci.PhotoURL
		happy := false
		if purl == nil || *purl == "" {
			// this is a no-op
			return "redirect", "/profile?tgt=" + url.QueryEscape(target)
		}
		if strings.HasPrefix(*purl, "/img/store/") {
			id, err := strconv.Atoi((*purl)[11:])
			if err != nil {
				return "error", err
			}
			defer func() {
				if happy {
					ampool.Submit(func(context.Context) {
						err := database.AmDeleteImage(ctxt.Ctx(), int32(id))
						if err != nil {
							log.Errorf("unable to delete image ID %d: %v", id, err)
						}
					})
				}
			}()
		}
		ci.PhotoURL = nil
		_, err := ci.Save(ctxt.Ctx())
		if err != nil {
			return "error", err
		}
		happy = true
		return "redirect", "/profile?tgt=" + url.QueryEscape(target)
	}
	return "error", EBUTTON
}

/* ShowProfile displays a user's profile.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func ShowProfile(ctxt ui.AmContext) (string, any) {
	me := ctxt.CurrentUser()
	prefs, err := me.Prefs(ctxt.Ctx())
	if err != nil {
		return "error", err
	}

	// Gather the info on the current user.
	user, err := database.AmGetUserByName(ctxt.Ctx(), ctxt.URLParam("uname"), nil)
	if err != nil {
		return "error", echo.NewHTTPError(http.StatusNotFound).SetInternal(err)
	}
	ci, err := user.ContactInfo(ctxt.Ctx())
	if err != nil {
		return "error", err
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
		ctxt.VarMap().Set("country", country.Emoji()+" "+country.String())
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
	ctxt.SetFrameTitle(fmt.Sprintf("User Profile - %s", user.Username))
	return "framed", "profile.jet"
}

/* QuickEMail sends quick E-mail to a user.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func QuickEMail(ctxt ui.AmContext) (string, any) {
	me := ctxt.CurrentUser()
	if me.IsAnon {
		return "error", ELOGIN
	}
	myCI, err := me.ContactInfo(ctxt.Ctx())
	if err != nil {
		return "error", err
	}
	toUid, err := ctxt.FormFieldInt("to_uid")
	if err != nil {
		return "error", err
	}
	user, err := database.AmGetUser(ctxt.Ctx(), int32(toUid))
	if err != nil {
		return "error", err
	}
	if user.IsAnon {
		return "error", "cannot send quick E-mail to anonymous user"
	}
	ci, err := user.ContactInfo(ctxt.Ctx())
	if err != nil {
		return "error", err
	}
	msg := email.AmNewEmailMessage(me.Uid, ctxt.RemoteIP())
	msg.AddTo(*ci.Email, user.Username)
	msg.AddHeader("X-Originally-From", fmt.Sprintf("%s <%s>", me.Username, *myCI.Email))
	msg.SetSubject(ctxt.FormField("subj"))
	msg.SetText(ctxt.FormField("pb"))
	msg.Send()
	return "redirect", "/user/" + user.Username
}

/* Hotlist displays and edits the user's conference hotlist.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func Hotlist(ctxt ui.AmContext) (string, any) {
	me := ctxt.CurrentUser()
	if me.IsAnon {
		return "error", ELOGIN
	}
	hotlist, err := database.AmGetConferenceHotlist(ctxt.Ctx(), me)
	if err != nil {
		return "error", err
	}

	if ctxt.HasParameter("m") {
		index := ctxt.QueryParamInt("m", -1)
		dir := ctxt.QueryParamInt("n", 0)
		if index >= 0 && (index+dir) != index {
			err := database.AmReorderHotlist(ctxt.Ctx(), me, hotlist[index].Sequence, hotlist[index+dir].Sequence)
			if err != nil {
				return "error", err
			}
			tmp := hotlist[index].CommId
			hotlist[index].CommId = hotlist[index+dir].CommId
			hotlist[index+dir].CommId = tmp
			tmp = hotlist[index].ConfId
			hotlist[index].ConfId = hotlist[index+dir].ConfId
			hotlist[index+dir].ConfId = tmp
		}
	} else if ctxt.HasParameter("d") {
		index := ctxt.QueryParamInt("d", -1)
		if index >= 0 {
			err := database.AmRemoveEntryFromHotlist(ctxt.Ctx(), me, hotlist[index].Sequence)
			if err != nil {
				return "error", err
			}
			hotlist = append(hotlist[:index], hotlist[index+1:]...)
		}
	}

	communities := make([]string, len(hotlist))
	conferences := make([]string, len(hotlist))
	for i := range hotlist {
		comm, err := hotlist[i].Community(ctxt.Ctx())
		if err != nil {
			return "error", err
		}
		communities[i] = comm.Name
		conf, err := hotlist[i].Conference(ctxt.Ctx())
		if err != nil {
			return "error", err
		}
		conferences[i] = conf.Name
	}

	ctxt.VarMap().Set("hotlist", hotlist)
	ctxt.VarMap().Set("communities", communities)
	ctxt.VarMap().Set("conferences", conferences)
	ctxt.SetFrameTitle("Your Conference Hotlist")
	return "framed", "hotlist.jet"
}

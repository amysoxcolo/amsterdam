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
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"git.erbosoft.com/amy/amsterdam/database"
	"git.erbosoft.com/amy/amsterdam/ui"
	"git.erbosoft.com/amy/amsterdam/util"
	log "github.com/sirupsen/logrus"
)

/* SysAdminMenu renders the system administration menu.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func SysAdminMenu(ctxt ui.AmContext) (string, any) {
	if !database.AmTestPermission("Global.SysAdminAccess", ctxt.CurrentUser().BaseLevel) {
		return "error", ENOACCESS
	}
	menu := ui.AmMenu("sysadmin")
	ctxt.VarMap().Set("menu", menu)
	ctxt.VarMap().Set("defs", make(map[string]bool))
	ctxt.SetFrameTitle(menu.Title)
	return "framed", "menu.jet"
}

/* GlobalPropertiesForm displays the global properties editing form.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func GlobalPropertiesForm(ctxt ui.AmContext) (string, any) {
	if !database.AmTestPermission("Global.SysAdminAccess", ctxt.CurrentUser().BaseLevel) {
		return "error", ENOACCESS
	}

	dlg, err := ui.AmLoadDialog("globalprops")
	if err != nil {
		return "error", err
	}
	dlg.Field("search_items").SetInt(int(ctxt.Globals().MaxSearchPage))
	dlg.Field("fp_posts").SetInt(int(ctxt.Globals().FrontPagePosts))
	dlg.Field("audit_recs").SetInt(int(ctxt.Globals().NumAuditPage))
	dlg.Field("create_lvl").SetLevel(uint16(ctxt.Globals().CommunityCreateLevel))
	dlg.Field("comm_mbrs").SetInt(int(ctxt.Globals().MaxCommunityMemberPage))
	dlg.Field("no_cats").SetChecked(ctxt.GlobalFlags().Get(database.GlobalFlagNoCategories))
	dlg.Field("posts_page").SetInt(int(ctxt.Globals().PostsPerPage))
	dlg.Field("old_posts").SetInt(int(ctxt.Globals().OldPostsAtTop))
	dlg.Field("conf_mbrs").SetInt(int(ctxt.Globals().MaxConferenceMemberPage))
	dlg.Field("pic_in_post").SetChecked(ctxt.GlobalFlags().Get(database.GlobalFlagPicturesInPosts))
	return dlg.Render(ctxt)
}

/* GlobalPropertiesSet resets the global properties.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func GlobalPropertiesSet(ctxt ui.AmContext) (string, any) {
	if !database.AmTestPermission("Global.SysAdminAccess", ctxt.CurrentUser().BaseLevel) {
		return "error", ENOACCESS
	}

	dlg, err := ui.AmLoadDialog("globalprops")
	if err != nil {
		return "error", err
	}
	dlg.LoadFromForm(ctxt)
	b := dlg.WhichButton(ctxt)
	if b == "cancel" {
		return "redirect", "/sysadmin"
	} else if b != "update" {
		return dlg.RenderError(ctxt, EBUTTON.Error())
	}

	gl, err := database.AmGlobals(ctxt.Ctx())
	if err != nil {
		return dlg.RenderError(ctxt, err.Error())
	}
	gl = gl.Clone()
	var n int
	if n, err = dlg.Field("search_items").ValueInt(); err != nil {
		return dlg.RenderError(ctxt, err.Error())
	}
	gl.MaxSearchPage = int32(n)
	if n, err = dlg.Field("fp_posts").ValueInt(); err != nil {
		return dlg.RenderError(ctxt, err.Error())
	}
	gl.FrontPagePosts = int32(n)
	if n, err = dlg.Field("audit_recs").ValueInt(); err != nil {
		return dlg.RenderError(ctxt, err.Error())
	}
	gl.NumAuditPage = int32(n)
	gl.CommunityCreateLevel = int32(dlg.Field("create_lvl").GetLevel())
	if n, err = dlg.Field("comm_mbrs").ValueInt(); err != nil {
		return dlg.RenderError(ctxt, err.Error())
	}
	gl.MaxCommunityMemberPage = int32(n)
	if n, err = dlg.Field("posts_page").ValueInt(); err != nil {
		return dlg.RenderError(ctxt, err.Error())
	}
	gl.PostsPerPage = int32(n)
	if n, err = dlg.Field("old_posts").ValueInt(); err != nil {
		return dlg.RenderError(ctxt, err.Error())
	}
	gl.OldPostsAtTop = int32(n)
	if n, err = dlg.Field("conf_mbrs").ValueInt(); err != nil {
		return dlg.RenderError(ctxt, err.Error())
	}
	gl.MaxConferenceMemberPage = int32(n)

	flags, err := gl.Flags(ctxt.Ctx())
	if err != nil {
		return dlg.RenderError(ctxt, err.Error())
	}
	flags.Set(database.GlobalFlagNoCategories, dlg.Field("no_cats").IsChecked())
	flags.Set(database.GlobalFlagPicturesInPosts, dlg.Field("pic_in_post").IsChecked())

	err = database.AmReplaceGlobals(ctxt.Ctx(), gl)
	if err == nil {
		err = gl.SaveFlags(ctxt.Ctx(), flags)
	}
	if err != nil {
		return "error", err
	}
	return "redirect", "/sysadmin"
}

/* UserManagementSearch displays the user management page and performs searches.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func UserManagementSearch(ctxt ui.AmContext) (string, any) {
	if !database.AmTestPermission("Global.SysAdminAccess", ctxt.CurrentUser().BaseLevel) {
		return "error", ENOACCESS
	}

	field := "name"
	oper := "st"
	term := ""
	ofs := 0
	doSearch := false
	listMax := int(ctxt.Globals().MaxSearchPage)
	if ctxt.Verb() == "POST" {
		field = ctxt.FormField("field")
		oper = ctxt.FormField("oper")
		term = ctxt.FormField("term")
		ofsStr := ctxt.FormField("ofs")
		if n, err := strconv.Atoi(ofsStr); err == nil {
			ofs = n
		}
		if ctxt.FormFieldIsSet("prev") {
			ofs = max(0, ofs-listMax)
		} else if ctxt.FormFieldIsSet("next") {
			ofs += listMax
		}
		doSearch = true
	}
	ctxt.VarMap().Set("field", field)
	ctxt.VarMap().Set("oper", oper)
	ctxt.VarMap().Set("term", term)
	ctxt.VarMap().Set("ofs", ofs)
	if doSearch {
		ulist, total, err := database.AmSearchUsers(ctxt.Ctx(), SearchUserFieldMap[field], SearchUserOperMap[oper], term, ofs, listMax)
		if err == nil {
			resultLine := ""
			if len(ulist) == 0 {
				resultLine = "None found"
			} else {
				resultLine = fmt.Sprintf("Displaying %d-%d of %d", ofs+1, ofs+len(ulist), total)
			}
			ctxt.VarMap().Set("resultHeader", resultLine)
			if len(ulist) > 0 {
				ctxt.VarMap().Set("resultList", ulist)
				if ofs > 0 {
					ctxt.VarMap().Set("resultShowPrev", true)
				}
				if (ofs + listMax) < total {
					ctxt.VarMap().Set("resultShowNext", true)
				}
			}
		} else {
			ctxt.VarMap().Set("errorMessage", err.Error())
		}
	}
	ctxt.SetFrameTitle("User Account Management")
	return "framed", "admin_users.jet"
}

/* UserManagementForm displays the form for modifying a user.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func UserManagementForm(ctxt ui.AmContext) (string, any) {
	if !database.AmTestPermission("Global.SysAdminAccess", ctxt.CurrentUser().BaseLevel) {
		return "error", ENOACCESS
	}
	user, err := database.AmGetUserByName(ctxt.Ctx(), ctxt.URLParam("uname"), nil)
	if err != nil {
		return "error", err
	}

	dlg, err := ui.AmLoadDialog("admin_user")
	if err == nil {
		dlg.SetTargetUser(user)
		if ctxt.CurrentUser().BaseLevel == database.AmRole("Global.BOFH").Level() {
			// only the BOFH can designate a user as a PFY!
			dlg.Field("base_lvl").Param = "Global.UserLevelsPFY"
		}
		var ci *database.ContactInfo
		ci, err = user.ContactInfo(ctxt.Ctx())
		if err == nil {
			var prefs *database.UserPrefs
			prefs, err = user.Prefs(ctxt.Ctx())
			if err == nil {
				dlg.Field("remind").Value = user.PassReminder
				dlg.Field("base_lvl").SetLevel(user.BaseLevel)
				dlg.Field("verify_email").SetChecked(user.VerifyEMail)
				dlg.Field("lockout").SetChecked(user.Lockout)
				dlg.Field("nophoto").SetChecked(user.FlagValue(ctxt.Ctx(), database.UserFlagDisallowSetPhoto))
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
				dlg.Field("dob").SetDate(user.DOB)
				dlg.Field("descr").SetVal(user.Description)
				dlg.Field("photo").Value = userPhotoURL(ci)
				dlg.Field("pic_in_post").SetChecked(user.FlagValue(ctxt.Ctx(), database.UserFlagPicturesInPosts))
				dlg.Field("no_mass_mail").SetChecked(user.FlagValue(ctxt.Ctx(), database.UserFlagMassMailOptOut))
				dlg.Field("locale").Value = prefs.ReadLocale()
				dlg.Field("tz").Value = prefs.TimeZoneID
				return dlg.Render(ctxt)
			}
		}
	}
	return "error", err
}

/* UserManagementSave saves the profile data of the user.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func UserManagementSave(ctxt ui.AmContext) (string, any) {
	if !database.AmTestPermission("Global.SysAdminAccess", ctxt.CurrentUser().BaseLevel) {
		return "error", ENOACCESS
	}
	user, err := database.AmGetUserByName(ctxt.Ctx(), ctxt.URLParam("uname"), nil)
	if err != nil {
		return "error", err
	}

	dlg, err := ui.AmLoadDialog("admin_user")
	if err == nil {
		dlg.LoadFromForm(ctxt)
		dlg.SetTargetUser(user)
		if ctxt.CurrentUser().BaseLevel == database.AmRole("Global.BOFH").Level() {
			// only the BOFH can designate a user as a PFY!
			dlg.Field("base_lvl").Param = "Global.UserLevelsPFY"
		}
		action := dlg.WhichButton(ctxt)
		if action == "cancel" { // Cancel button pressed
			return "redirect", "/sysadmin/users"
		}
		if action == "update" {
			err = dlg.Validate()
			if err != nil {
				return dlg.RenderError(ctxt, err.Error())
			}
			var ci *database.ContactInfo
			ci, err = user.ContactInfo(ctxt.Ctx())
			if err == nil {
				var prefs *database.UserPrefs
				prefs, err = user.Prefs(ctxt.Ctx())
				if err == nil && !(dlg.Field("pass1").IsEmpty() && dlg.Field("pass2").IsEmpty()) {
					p1 := dlg.Field("pass1").Value
					if p1 == dlg.Field("pass2").Value {
						err = user.ChangePassword(ctxt.Ctx(), p1, ctxt.CurrentUser(), ctxt.RemoteIP())
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
					_, err = nci.Save(ctxt.Ctx(), ctxt.CurrentUser(), ctxt.RemoteIP())
					ci = nci
				}
				if err == nil {
					nprefs := prefs.Clone()
					nprefs.WriteLocale(dlg.Field("locale").Value)
					nprefs.TimeZoneID = dlg.Field("tz").Value
					err = nprefs.Save(ctxt.Ctx(), user, ctxt.CurrentUser(), ctxt.RemoteIP())
				}
				if err == nil {
					var f *util.OptionSet
					f, err = user.Flags(ctxt.Ctx())
					if err == nil {
						nf := f.Clone()
						nf.Set(database.UserFlagPicturesInPosts, dlg.Field("pic_in_post").IsChecked())
						nf.Set(database.UserFlagMassMailOptOut, dlg.Field("no_mass_mail").IsChecked())
						nf.Set(database.UserFlagDisallowSetPhoto, dlg.Field("nophoto").IsChecked())
						err = user.SaveFlags(ctxt.Ctx(), nf)
					}
				}
				if err == nil {
					err = user.SetProfileData(ctxt.Ctx(), dlg.Field("remind").Value, dlg.Field("dob").AsDate(), dlg.Field("descr").ValPtr(),
						ctxt.CurrentUser(), ctxt.RemoteIP())
				}
				if err == nil {
					err = user.SetSecurityData(ctxt.Ctx(), dlg.Field("base_lvl").GetLevel(), dlg.Field("lockout").IsChecked(),
						dlg.Field("verify_email").IsChecked(), ctxt.CurrentUser(), ctxt.RemoteIP())
				}
				if err == nil {
					return "redirect", "/sysadmin/users"
				}
			}
		}
		return dlg.RenderError(ctxt, EBUTTON.Error())
	}
	return "error", err
}

/* AdminUserPhotoForm displays the form for editing the user's photo.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func AdminUserPhotoForm(ctxt ui.AmContext) (string, any) {
	if !database.AmTestPermission("Global.SysAdminAccess", ctxt.CurrentUser().BaseLevel) {
		return "error", ENOACCESS
	}
	user, err := database.AmGetUserByName(ctxt.Ctx(), ctxt.URLParam("uname"), nil)
	if err != nil {
		return "error", err
	}
	ci, err := user.ContactInfo(ctxt.Ctx())
	if err == nil {
		ctxt.VarMap().Set("target", "")
		ctxt.VarMap().Set("username", user.Username)
		ctxt.VarMap().Set("postUrl", fmt.Sprintf("/sysadmin/users/%s/photo", user.Username))
		ctxt.VarMap().Set("photo_url", userPhotoURL(ci))
		ctxt.SetFrameTitle("Upload User Photo for: " + user.Username)
		return "framed", "photo_upload.jet"
	}
	return "error", err
}

/* AdminUserPhoto handles processing the user's photo.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func AdminUserPhoto(ctxt ui.AmContext) (string, any) {
	if !database.AmTestPermission("Global.SysAdminAccess", ctxt.CurrentUser().BaseLevel) {
		return "error", ENOACCESS
	}
	user, err := database.AmGetUserByName(ctxt.Ctx(), ctxt.URLParam("uname"), nil)
	if err != nil {
		return "error", err
	}
	ci, err := user.ContactInfo(ctxt.Ctx())
	if err != nil {
		return "error", err
	}
	if ctxt.FormFieldIsSet("cancel") {
		return "redirect", fmt.Sprintf("/sysadmin/users/%s", user.Username)
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
				img, err = database.AmStoreImage(ctxt.Ctx(), database.ImageTypeUserPhoto, user.Uid, mimeType, imageData)
				if err == nil {
					photourl := fmt.Sprintf("/img/store/%d", img.ImgId)
					ci.PhotoURL = &photourl
					_, err = ci.Save(ctxt.Ctx(), ctxt.CurrentUser(), ctxt.RemoteIP())
					if err == nil {
						return "redirect", fmt.Sprintf("/sysadmin/users/%s", user.Username)
					}
				}
			}
		}
		ctxt.VarMap().Set("errorMessage", err.Error())
		ctxt.VarMap().Set("target", "")
		ctxt.VarMap().Set("username", user.Username)
		ctxt.VarMap().Set("postUrl", fmt.Sprintf("/sysadmin/users/%s/photo", user.Username))
		ctxt.VarMap().Set("photo_url", userPhotoURL(ci))
		ctxt.SetFrameTitle("Upload User Photo for: " + user.Username)
		return "framed", "photo_upload.jet"
	}
	if ctxt.FormFieldIsSet("remove") {
		purl := ci.PhotoURL
		happy := false
		if purl == nil || *purl == "" {
			// this is a no-op
			return "redirect", fmt.Sprintf("/sysadmin/users/%s", user.Username)
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
		_, err := ci.Save(ctxt.Ctx(), ctxt.CurrentUser(), ctxt.RemoteIP())
		if err != nil {
			return "error", err
		}
		happy = true
		return "redirect", fmt.Sprintf("/sysadmin/users/%s", user.Username)
	}
	return "error", EBUTTON
}

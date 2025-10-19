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
	"strconv"
	"strings"

	"git.erbosoft.com/amy/amsterdam/database"
	"git.erbosoft.com/amy/amsterdam/ui"
	"git.erbosoft.com/amy/amsterdam/util"
	log "github.com/sirupsen/logrus"
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

func setupCommunityProfileDialog(dlg *ui.Dialog, comm *database.Community) {
	dlg.SetCommunity(comm)
	if comm.IsAdmin {
		dlg.Field("comtype").Disabled = true
		dlg.Field("joinkey").Disabled = true
		dlg.Field("membersonly").Disabled = true
		dlg.Field("hidemode").Disabled = true
		dlg.Field("read_lvl").Disabled = true
		dlg.Field("write_lvl").Disabled = true
		dlg.Field("create_lvl").Disabled = true
		dlg.Field("delete_lvl").Disabled = true
		dlg.Field("join_lvl").Disabled = true
	}
}

// communityLogoURL returns the logo URL from the contact info, or a default.
func communityLogoURL(ci *database.ContactInfo) string {
	if ci.PhotoURL != nil && *ci.PhotoURL != "" {
		return *ci.PhotoURL
	}
	return "/img/builtin/default-community.jpg"
}

/* CommunityProfileForm displays the dfialog for editing the community profile.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func CommunityProfileForm(ctxt ui.AmContext) (string, any, error) {
	err := ctxt.SetCommunityContext(ctxt.URLParam("cid"))
	if err != nil {
		ctxt.SetRC(http.StatusNotFound)
		return ui.ErrorPage(ctxt, err)
	}
	comm := ctxt.CurrentCommunity()
	if !comm.TestPermission("Community.Write", ctxt.EffectiveLevel()) {
		ctxt.SetRC(http.StatusForbidden)
		return ui.ErrorPage(ctxt, errors.New("you are not permitted to access this page"))
	}
	var ci *database.ContactInfo
	ci, err = comm.ContactInfo()
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	flags, err := comm.Flags()
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	dlg, err := ui.AmLoadDialog("commprofile")
	if err == nil {
		setupCommunityProfileDialog(dlg, comm)
		dlg.Field("cc").Value = fmt.Sprintf("%d", comm.Id)
		dlg.Field("name").Value = comm.Name
		dlg.Field("alias").Value = comm.Alias
		dlg.Field("synopsis").SetVal(comm.Synopsis)
		dlg.Field("rules").SetVal(comm.Rules)
		dlg.Field("language").SetVal(comm.Language)
		dlg.Field("url").SetVal(ci.URL)
		dlg.Field("logo").Value = communityLogoURL(ci)
		dlg.Field("company").SetVal(ci.Company)
		dlg.Field("addr1").SetVal(ci.Addr1)
		dlg.Field("addr2").SetVal(ci.Addr2)
		dlg.Field("loc").SetVal(ci.Locality)
		dlg.Field("reg").SetVal(ci.Region)
		dlg.Field("pcode").SetVal(ci.PostalCode)
		dlg.Field("country").SetVal(ci.Country)
		if comm.Public() {
			dlg.Field("comtype").Value = "0"
			dlg.Field("joinkey").Value = ""
		} else {
			dlg.Field("comtype").Value = "1"
			dlg.Field("joinkey").SetVal(comm.JoinKey)
		}
		dlg.Field("membersonly").SetChecked(comm.MembersOnly)
		dlg.Field("hidemode").Value = comm.HideMode()
		dlg.Field("read_lvl").Value = fmt.Sprintf("%d", comm.ReadLevel)
		dlg.Field("write_lvl").Value = fmt.Sprintf("%d", comm.WriteLevel)
		dlg.Field("create_lvl").Value = fmt.Sprintf("%d", comm.CreateLevel)
		dlg.Field("delete_lvl").Value = fmt.Sprintf("%d", comm.DeleteLevel)
		dlg.Field("join_lvl").Value = fmt.Sprintf("%d", comm.JoinLevel)
		dlg.Field("pic_in_post").SetChecked(flags.Get(database.CommunityFlagPicturesInPosts))
		return dlg.Render(ctxt)
	}
	return ui.ErrorPage(ctxt, err)
}

// levelFld is a quick routine to extract a level value from a drop-down.
func levelFld(d *ui.Dialog, name string) uint16 {
	v, _ := strconv.Atoi(d.Field(name).Value)
	return uint16(v)
}

/* EditCommunityProfile updates the community's profile from the dialog.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func EditCommunityProfile(ctxt ui.AmContext) (string, any, error) {
	err := ctxt.SetCommunityContext(ctxt.URLParam("cid"))
	if err != nil {
		ctxt.SetRC(http.StatusNotFound)
		return ui.ErrorPage(ctxt, err)
	}
	comm := ctxt.CurrentCommunity()
	if !comm.TestPermission("Community.Write", ctxt.EffectiveLevel()) {
		ctxt.SetRC(http.StatusForbidden)
		return ui.ErrorPage(ctxt, errors.New("you are not permitted to access this page"))
	}
	dlg, err := ui.AmLoadDialog("commprofile")
	if err == nil {
		setupCommunityProfileDialog(dlg, comm)
		dlg.LoadFromForm(ctxt)

		action := dlg.WhichButton(ctxt)
		if action == "cancel" {
			return "redirect", fmt.Sprintf("/comm/%s/admin", comm.Alias), nil
		}
		if action == "update" {
			err = dlg.Validate()
			if err != nil {
				return dlg.RenderError(ctxt, err.Error())
			}
			var ci *database.ContactInfo
			ci, err = comm.ContactInfo()
			if err != nil {
				return dlg.RenderError(ctxt, err.Error())
			}
			var flags *util.OptionSet
			flags, err = comm.Flags()
			if err != nil {
				return ui.ErrorPage(ctxt, err)
			}
			nci := ci.Clone()
			nci.URL = dlg.Field("url").ValPtr()
			nci.Company = dlg.Field("company").ValPtr()
			nci.Addr1 = dlg.Field("addr1").ValPtr()
			nci.Addr2 = dlg.Field("addr2").ValPtr()
			nci.Locality = dlg.Field("loc").ValPtr()
			nci.Region = dlg.Field("reg").ValPtr()
			nci.PostalCode = dlg.Field("pcode").ValPtr()
			nci.Country = dlg.Field("country").ValPtr()
			_, err = nci.Save()
			ci = nci
			if err == nil {
				var joinkey *string = nil
				if dlg.Field("comtype").Value == "1" {
					joinkey = dlg.Field("joinkey").ValPtr()
				}
				var hidedir, hidesearch bool
				switch dlg.Field("hidemode").Value {
				case "NONE":
					hidedir = false
					hidesearch = false
				case "DIRECTORY":
					hidedir = false
					hidesearch = true
				case "BOTH":
					hidedir = true
					hidesearch = true
				}
				err = comm.SetProfileData(dlg.Field("name").Value, dlg.Field("alias").Value, dlg.Field("synopsis").ValPtr(),
					dlg.Field("rules").ValPtr(), dlg.Field("language").ValPtr(), joinkey, dlg.Field("membersonly").IsChecked(),
					hidedir, hidesearch, levelFld(dlg, "read_lvl"), levelFld(dlg, "write_lvl"), levelFld(dlg, "create_lvl"),
					levelFld(dlg, "delete_lvl"), levelFld(dlg, "join_lvl"))
			}
			if err == nil {
				flags.Set(database.CommunityFlagPicturesInPosts, dlg.Field("pic_in_post").IsChecked())
				err = comm.SaveFlags(flags)
			}
			if err != nil {
				ctxt.ClearCommunityContext()
				return dlg.RenderError(ctxt, err.Error())
			} else {
				return "redirect", fmt.Sprintf("/comm/%s/admin", comm.Alias), nil
			}
		}
		return dlg.RenderError(ctxt, "No known button click on POST to community profile.")
	}
	return ui.ErrorPage(ctxt, err)
}

func CommunityLogoForm(ctxt ui.AmContext) (string, any, error) {
	err := ctxt.SetCommunityContext(ctxt.URLParam("cid"))
	if err != nil {
		ctxt.SetRC(http.StatusNotFound)
		return ui.ErrorPage(ctxt, err)
	}
	comm := ctxt.CurrentCommunity()
	if !comm.TestPermission("Community.Write", ctxt.EffectiveLevel()) {
		ctxt.SetRC(http.StatusForbidden)
		return ui.ErrorPage(ctxt, errors.New("you are not permitted to access this page"))
	}
	ci, err := comm.ContactInfo()
	if err == nil {
		ctxt.VarMap().Set("commName", comm.Name)
		ctxt.VarMap().Set("commAlias", comm.Alias)
		ctxt.VarMap().Set("logo_url", communityLogoURL(ci))
		ctxt.VarMap().Set("amsterdam_pageTitle", "Upload Community Logo: "+comm.Name)
		return "framed_template", "logo_upload.jet", nil
	}
	return ui.ErrorPage(ctxt, err)
}

func EditCommunityLogo(ctxt ui.AmContext) (string, any, error) {
	err := ctxt.SetCommunityContext(ctxt.URLParam("cid"))
	if err != nil {
		ctxt.SetRC(http.StatusNotFound)
		return ui.ErrorPage(ctxt, err)
	}
	comm := ctxt.CurrentCommunity()
	if !comm.TestPermission("Community.Write", ctxt.EffectiveLevel()) {
		ctxt.SetRC(http.StatusForbidden)
		return ui.ErrorPage(ctxt, errors.New("you are not permitted to access this page"))
	}
	ci, err := comm.ContactInfo()
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	if ctxt.FormFieldIsSet("cancel") {
		return "redirect", "/comm/" + comm.Alias + "/admin/profile", nil
	}
	if ctxt.FormFieldIsSet("upload") {
		file, err := ctxt.FormFile("thepic")
		if err == nil {
			var imageData []byte
			var mimeType string
			imageData, mimeType, err = ui.AmProcessUploadedImage(file, ui.CommunityLogoWidth, ui.CommunityLogoHeight,
				ui.CommunityLogoMaxBytes)
			if err == nil {
				var img *database.ImageStore
				img, err = database.AmStoreImage(database.ImageTypeCommunityLogo, comm.Id, mimeType, imageData)
				if err == nil {
					photourl := fmt.Sprintf("/img/store/%d", img.ImgId)
					ci.PhotoURL = &photourl
					_, err = ci.Save()
					if err == nil {
						return "redirect", "/comm/" + comm.Alias + "/admin/profile", nil
					}
				}
			}
		}
		ctxt.VarMap().Set("errorMessage", err.Error())
		ctxt.VarMap().Set("commName", comm.Name)
		ctxt.VarMap().Set("commAlias", comm.Alias)
		ctxt.VarMap().Set("logo_url", communityLogoURL(ci))
		ctxt.VarMap().Set("amsterdam_pageTitle", "Upload Community Logo: "+comm.Name)
		return "framed_template", "logo_upload.jet", nil
	}
	if ctxt.FormFieldIsSet("remove") {
		purl := ci.PhotoURL
		happy := false
		if purl == nil || *purl == "" {
			// this is a no-op
			return "redirect", "/comm/" + comm.Alias + "/admin/profile", nil
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
		return "redirect", "/comm/" + comm.Alias + "/admin/profile", nil
	}
	return ui.ErrorPage(ctxt, errors.New("invalid button detected in logo upload"))
}

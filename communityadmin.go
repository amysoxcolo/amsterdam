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
		// TODO: set logo URL
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

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
	"fmt"
	"strconv"
	"strings"

	"git.erbosoft.com/amy/amsterdam/database"
	"git.erbosoft.com/amy/amsterdam/ui"
	"git.erbosoft.com/amy/amsterdam/util"
	log "github.com/sirupsen/logrus"
)

/* EditConferenceForm displays the dialog for editing the conference properties.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func EditConferenceForm(ctxt ui.AmContext) (string, any) {
	comm := ctxt.CurrentCommunity()
	conf := ctxt.GetScratch("currentConference").(*database.Conference)
	myLevel := ctxt.GetScratch("levelInConference").(uint16)
	if !conf.TestPermission("Conference.Change", myLevel) {
		return "error", ENOPERM
	}

	dlg, err := ui.AmLoadDialog("edit_conference")
	if err != nil {
		return "error", err
	}
	dlg.SetCommunity(comm)
	dlg.SetConference(conf, ctxt.GetScratch("currentAlias").(string))
	dlg.Field("name").Value = conf.Name
	dlg.Field("descr").SetVal(conf.Description)
	if comm.TestPermission("Community.Create", ctxt.EffectiveLevel()) {
		f, err := conf.HiddenInList(ctxt.Ctx(), comm)
		if err != nil {
			return "error", err
		}
		dlg.Field("hide").SetChecked(f)
	} else {
		dlg.Field("hide").Disabled = true
	}
	dlg.Field("read_lvl").SetLevel(conf.ReadLevel)
	dlg.Field("post_lvl").SetLevel(conf.PostLevel)
	dlg.Field("create_lvl").SetLevel(conf.CreateLevel)
	dlg.Field("hide_lvl").SetLevel(conf.HideLevel)
	dlg.Field("nuke_lvl").SetLevel(conf.NukeLevel)
	dlg.Field("change_lvl").SetLevel(conf.ChangeLevel)
	dlg.Field("delete_lvl").SetLevel(conf.DeleteLevel)
	flags, err := conf.Flags(ctxt.Ctx())
	if err != nil {
		return "error", err
	}
	dlg.Field("pic_in_post").SetChecked(flags.Get(database.ConferenceFlagPicturesInPosts))
	return dlg.Render(ctxt)
}

/* EditConference saves the conference properties being edited.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func EditConference(ctxt ui.AmContext) (string, any) {
	comm := ctxt.CurrentCommunity()
	conf := ctxt.GetScratch("currentConference").(*database.Conference)
	myLevel := ctxt.GetScratch("levelInConference").(uint16)
	if !conf.TestPermission("Conference.Change", myLevel) {
		return "error", ENOPERM
	}

	dlg, err := ui.AmLoadDialog("edit_conference")
	if err != nil {
		return "error", err
	}
	button := dlg.WhichButton(ctxt)
	if button == "cancel" {
		return "redirect", fmt.Sprintf("/comm/%s/conf/%s/manage", comm.Alias, ctxt.GetScratch("currentAlias"))
	} else if button != "update" {
		dlg.SetCommunity(comm)
		dlg.SetConference(conf, ctxt.GetScratch("currentAlias").(string))
		return dlg.RenderError(ctxt, "invalid button pressed")
	}

	dlg.LoadFromForm(ctxt)
	if err = dlg.Validate(); err == nil {
		if err = conf.SetInfo(ctxt.Ctx(), dlg.Field("name").Value, dlg.Field("descr").Value, dlg.Field("read_lvl").GetLevel(), dlg.Field("post_lvl").GetLevel(),
			dlg.Field("create_lvl").GetLevel(), dlg.Field("hide_lvl").GetLevel(), dlg.Field("nuke_lvl").GetLevel(), dlg.Field("change_lvl").GetLevel(),
			dlg.Field("delete_lvl").GetLevel()); err == nil {
			if err = conf.SetHiddenInList(ctxt.Ctx(), comm, dlg.Field("hide").IsChecked()); err == nil {
				var flags *util.OptionSet
				flags, err = conf.Flags(ctxt.Ctx())
				if err == nil {
					flags.Set(database.ConferenceFlagPicturesInPosts, dlg.Field("pic_in_post").IsChecked())
					err = conf.SaveFlags(ctxt.Ctx(), flags)
				}
			}
		}
	}
	if err != nil {
		dlg.SetCommunity(comm)
		dlg.SetConference(conf, ctxt.GetScratch("currentAlias").(string))
		return dlg.RenderError(ctxt, err.Error())
	}

	return "redirect", fmt.Sprintf("/comm/%s/conf/%s/manage", comm.Alias, ctxt.GetScratch("currentAlias"))
}

/* ConferenceAliasForm displays the form for managing conference aliases.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func ConferenceAliasForm(ctxt ui.AmContext) (string, any) {
	comm := ctxt.CurrentCommunity()
	conf := ctxt.GetScratch("currentConference").(*database.Conference)
	myLevel := ctxt.GetScratch("levelInConference").(uint16)
	if !conf.TestPermission("Conference.Change", myLevel) {
		return "error", ENOPERM
	}

	ctxt.VarMap().Set("newAlias", "")
	ctxt.VarMap().Set("confName", conf.Name)
	ctxt.VarMap().Set("backLink", fmt.Sprintf("/comm/%s/conf/%s/manage", comm.Alias, ctxt.GetScratch("currentAlias")))
	ctxt.VarMap().Set("selfLink", fmt.Sprintf("/comm/%s/conf/%s/aliases", comm.Alias, ctxt.GetScratch("currentAlias")))
	ctxt.SetFrameTitle(fmt.Sprintf("Manage Conference Aliases: %s", conf.Name))

	if ctxt.HasParameter("del") {
		err := conf.RemoveAlias(ctxt.Ctx(), ctxt.Parameter("del"), ctxt.CurrentUser(), ctxt.RemoteIP())
		if err != nil {
			ctxt.VarMap().Set("errorMessage", err.Error())
		}
	}

	aliases, err := conf.Aliases(ctxt.Ctx())
	if err != nil {
		return "error", err
	}

	ctxt.VarMap().Set("aliases", aliases)
	return "framed", "conf_aliases.jet"
}

/* ConferenceAliasAdd adds a new alias to the current conference.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func ConferenceAliasAdd(ctxt ui.AmContext) (string, any) {
	comm := ctxt.CurrentCommunity()
	conf := ctxt.GetScratch("currentConference").(*database.Conference)
	myLevel := ctxt.GetScratch("levelInConference").(uint16)
	if !conf.TestPermission("Conference.Change", myLevel) {
		return "error", ENOPERM
	}

	ctxt.VarMap().Set("confName", conf.Name)
	ctxt.VarMap().Set("backLink", fmt.Sprintf("/comm/%s/conf/%s/manage", comm.Alias, ctxt.GetScratch("currentAlias")))
	ctxt.VarMap().Set("selfLink", fmt.Sprintf("/comm/%s/conf/%s/aliases", comm.Alias, ctxt.GetScratch("currentAlias")))
	ctxt.SetFrameTitle(fmt.Sprintf("Manage Conference Aliases: %s", conf.Name))

	newAlias := ctxt.FormField("na")
	ctxt.VarMap().Set("newAlias", newAlias)

	var err error = nil
	if ctxt.FormFieldIsSet("add") {
		if database.AmIsValidAmsterdamID(newAlias) {
			err = conf.AddAlias(ctxt.Ctx(), newAlias, ctxt.CurrentUser(), ctxt.RemoteIP())
		} else {
			err = fmt.Errorf("value '%s' is not a valid Amsterdam id", newAlias)
		}
	} else {
		err = errors.New("invalid button press")
	}

	if err != nil {
		ctxt.VarMap().Set("errorMessage", err.Error())
	}

	aliases, err := conf.Aliases(ctxt.Ctx())
	if err != nil {
		return "error", err
	}

	ctxt.VarMap().Set("newAlias", "")
	ctxt.VarMap().Set("aliases", aliases)
	return "framed", "conf_aliases.jet"
}

// CMData is the result data passed to the conference members page.
type CMData struct {
	User  *database.User
	Level uint16
}

// fieldMap maps field names to search field indexes.
var fieldMap = map[string]int{
	"name":  database.SearchUserFieldName,
	"descr": database.SearchUserFieldDescription,
	"first": database.SearchUserFieldFirstName,
	"last":  database.SearchUserFieldLastName,
}

// operMap maps operator names to search operator indices.
var operMap = map[string]int{
	"st": database.SearchUserOperPrefix,
	"in": database.SearchUserOperSubstring,
	"re": database.SearchUserOperRegex,
}

/* ConferenceMembers shows the conference members and allows their access levels to be adjusted.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func ConferenceMembers(ctxt ui.AmContext) (string, any) {
	comm := ctxt.CurrentCommunity()
	conf := ctxt.GetScratch("currentConference").(*database.Conference)
	myLevel := ctxt.GetScratch("levelInConference").(uint16)
	if !conf.TestPermission("Conference.Change", myLevel) {
		return "error", ENOPERM
	}

	// Set the first batch of page variables.
	ctxt.VarMap().Set("commName", comm.Name)
	ctxt.VarMap().Set("confName", conf.Name)
	ctxt.VarMap().Set("backLink", fmt.Sprintf("/comm/%s/conf/%s/manage", comm.Alias, ctxt.GetScratch("currentAlias")))
	ctxt.VarMap().Set("selfLink", fmt.Sprintf("/comm/%s/conf/%s/members", comm.Alias, ctxt.GetScratch("currentAlias")))
	ctxt.VarMap().Set("roleList", database.AmRoleList("Conference.UserLevels"))
	ctxt.SetFrameTitle(fmt.Sprintf("Membership in Conference: %s", conf.Name))

	// Get the search parameter values and adjust them.
	mode := "conf"
	field := "name"
	oper := "st"
	term := ""
	offset := 0
	if ctxt.Verb() == "POST" {
		mode = ctxt.FormField("mode")
		field = ctxt.FormField("field")
		oper = ctxt.FormField("oper")
		term = ctxt.FormField("term")
		var e1 error
		offset, e1 = ctxt.FormFieldInt("ofs")
		if e1 != nil {
			offset = 0
		}
	}
	maxPage := ctxt.Globals().MaxSearchPage

	// Adjust the offset based on the page buttons.
	if ctxt.FormFieldIsSet("prev") {
		offset = max(0, offset-int(maxPage))
	} else if ctxt.FormFieldIsSet("next") {
		offset += int(maxPage)
	}

	// Write the search parameters back to the page variables.
	ctxt.VarMap().Set("mode", mode)
	ctxt.VarMap().Set("field", field)
	ctxt.VarMap().Set("oper", oper)
	ctxt.VarMap().Set("term", term)
	ctxt.VarMap().Set("offset", offset)
	ctxt.VarMap().Set("max", maxPage)

	if ctxt.FormFieldIsSet("update") {
		// Parse out the list of valid UIDs.
		uids := util.Map(strings.Split(ctxt.FormField("validUids"), "|"), func(in string) int32 {
			rc, err := strconv.Atoi(in)
			if err != nil {
				return -1
			}
			return int32(rc)
		})
		for _, uid := range uids {
			if uid > 0 {
				// Get old and new access levels from the form.
				tmp, err := ctxt.FormFieldInt(fmt.Sprintf("old_%d", uid))
				if err == nil {
					oldLevel := uint16(tmp)
					tmp, err = ctxt.FormFieldInt(fmt.Sprintf("new_%d", uid))
					if err == nil {
						newLevel := uint16(tmp)
						if oldLevel != newLevel {
							// Update the level for this user.
							var u *database.User
							u, err = database.AmGetUser(ctxt.Ctx(), uid)
							if err == nil {
								err = conf.SetMembership(ctxt.Ctx(), u, newLevel, ctxt.CurrentUser(), ctxt.RemoteIP())
							}
						}
					}
				}
				if err != nil {
					return "error", err
				}
			}
		}
		ctxt.VarMap().Set("updated", true)
	}

	// Get the member list for the conference.
	members, err := conf.Members(ctxt.Ctx())
	if err != nil {
		return "error", err
	}

	// Generate the result list.
	total := 0
	var mr []CMData
	switch mode {
	case "conf":
		total = len(members)
		if offset > 0 {
			members = members[offset:]
		}
		if len(members) > int(maxPage) {
			members = members[:maxPage]
		}
		mr = make([]CMData, len(members))
		for i := range members {
			mr[i].User, _ = database.AmGetUser(ctxt.Ctx(), members[i].Uid)
			mr[i].Level = members[i].Level
		}
	case "comm":
		ulist, t, err := database.AmSearchCommunityMembers(ctxt.Ctx(), comm, fieldMap[field], operMap[oper], term, offset, int(maxPage))
		if err != nil {
			return "error", err
		}
		total = t
		mr = make([]CMData, len(ulist))
		for i := range ulist {
			mr[i].User = ulist[i]
			mr[i].Level = 0
			for j := range members {
				if members[j].Uid == ulist[i].Uid {
					mr[i].Level = members[j].Level
					break
				}
			}
		}
	}

	// Set the last few variables and return.
	ctxt.VarMap().Set("resultList", mr)
	ctxt.VarMap().Set("total", total)
	ctxt.VarMap().Set("validUids", strings.Join(util.Map(mr, func(cd CMData) string {
		return fmt.Sprintf("%d", cd.User.Uid)
	}), "|"))
	if offset > 0 {
		ctxt.VarMap().Set("showPrev", true)
	}
	if (offset + len(mr)) < total {
		ctxt.VarMap().Set("showNext", true)
	}
	return "framed", "conf_members.jet"
}

/* ConfCustomForm displays the form for editing the conference's custom HTML blocks.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func ConfCustomForm(ctxt ui.AmContext) (string, any) {
	comm := ctxt.CurrentCommunity()
	conf := ctxt.GetScratch("currentConference").(*database.Conference)
	myLevel := ctxt.GetScratch("levelInConference").(uint16)
	if !conf.TestPermission("Conference.Change", myLevel) {
		return "error", ENOPERM
	}

	topBlock, bottomBlock, err := conf.GetCustomBlocks(ctxt.Ctx())
	if err != nil {
		return "error", err
	}

	ctxt.VarMap().Set("confName", conf.Name)
	ctxt.VarMap().Set("selfLink", fmt.Sprintf("/comm/%s/conf/%s/custom", comm.Alias, ctxt.GetScratch("currentAlias")))
	ctxt.VarMap().Set("topText", topBlock)
	ctxt.VarMap().Set("bottomText", bottomBlock)
	ctxt.SetFrameTitle(fmt.Sprintf("Customize Conference: %s", conf.Name))
	return "framed", "conf_custom.jet"
}

/* ConfCustom modifies or removes the conference's custom HTML blocks.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func ConfCustom(ctxt ui.AmContext) (string, any) {
	comm := ctxt.CurrentCommunity()
	conf := ctxt.GetScratch("currentConference").(*database.Conference)
	myLevel := ctxt.GetScratch("levelInConference").(uint16)
	if !conf.TestPermission("Conference.Change", myLevel) {
		return "error", ENOPERM
	}

	var err error
	if ctxt.FormFieldIsSet("cancel") {
		err = nil
	} else if ctxt.FormFieldIsSet("remove") {
		err = conf.RemoveCustomBlocks(ctxt.Ctx())
	} else if ctxt.FormFieldIsSet("update") {
		err = conf.SetCustomBlocks(ctxt.Ctx(), ctxt.FormField("tx"), ctxt.FormField("bx"))
	} else {
		return "error", EBUTTON
	}
	if err != nil {
		return "error", err
	}
	return "redirect", fmt.Sprintf("/comm/%s/conf/%s/manage", comm.Alias, ctxt.GetScratch("currentAlias"))
}

/* CreateConferenceForm displays the dialog for creating a new conference.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func CreateConferenceForm(ctxt ui.AmContext) (string, any) {
	comm := ctxt.CurrentCommunity()
	if !comm.TestPermission("Community.Create", ctxt.EffectiveLevel()) {
		return "error", ENOPERM
	}

	dlg, err := ui.AmLoadDialog("create_conference")
	if err != nil {
		return "error", err
	}
	dlg.SetCommunity(comm)
	return dlg.Render(ctxt)
}

/* CreateConference creates a new conference.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func CreateConference(ctxt ui.AmContext) (string, any) {
	comm := ctxt.CurrentCommunity()
	if !comm.TestPermission("Community.Create", ctxt.EffectiveLevel()) {
		return "error", ENOPERM
	}

	dlg, err := ui.AmLoadDialog("create_conference")
	if err != nil {
		return "error", err
	}
	button := dlg.WhichButton(ctxt)
	if button == "cancel" {
		return "redirect", fmt.Sprintf("/comm/%s/conf", comm.Alias)
	} else if button != "create" {
		dlg.SetCommunity(comm)
		return dlg.RenderError(ctxt, "invalid button pressed")
	}
	dlg.LoadFromForm(ctxt)
	alias := dlg.Field("alias").Value
	conf, err := database.AmCreateConference(ctxt.Ctx(), comm, dlg.Field("name").Value, alias, dlg.Field("descr").Value,
		dlg.Field("ctype").Value == "1", dlg.Field("hide").IsChecked(), ctxt.CurrentUser(), ctxt.RemoteIP())
	if err != nil {
		dlg.SetCommunity(comm)
		return dlg.RenderError(ctxt, err.Error())
	}
	log.Infof("Created conference '%s'", conf.Name)
	return "redirect", fmt.Sprintf("/comm/%s/conf/%s", comm.Alias, alias)
}

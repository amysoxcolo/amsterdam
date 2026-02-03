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
	"fmt"
	"net/http"

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
 *     Standard Go error status.
 */
func EditConferenceForm(ctxt ui.AmContext) (string, any, error) {
	comm := ctxt.CurrentCommunity()
	conf := ctxt.GetScratch("currentConference").(*database.Conference)
	myLevel := ctxt.GetScratch("levelInConference").(uint16)
	if !conf.TestPermission("Conference.Change", myLevel) {
		ctxt.SetRC(http.StatusForbidden)
		return ui.ErrorPage(ctxt, ENOPERM)
	}

	dlg, err := ui.AmLoadDialog("edit_conference")
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	dlg.SetCommunity(comm)
	dlg.SetConference(conf, ctxt.GetScratch("currentAlias").(string))
	dlg.Field("name").Value = conf.Name
	dlg.Field("descr").SetVal(conf.Description)
	if comm.TestPermission("Community.Create", ctxt.EffectiveLevel()) {
		f, err := conf.HiddenInList(ctxt.Ctx(), comm)
		if err != nil {
			return ui.ErrorPage(ctxt, err)
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
		return ui.ErrorPage(ctxt, err)
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
 *     Standard Go error status.
 */
func EditConference(ctxt ui.AmContext) (string, any, error) {
	comm := ctxt.CurrentCommunity()
	conf := ctxt.GetScratch("currentConference").(*database.Conference)
	myLevel := ctxt.GetScratch("levelInConference").(uint16)
	if !conf.TestPermission("Conference.Change", myLevel) {
		ctxt.SetRC(http.StatusForbidden)
		return ui.ErrorPage(ctxt, ENOPERM)
	}

	dlg, err := ui.AmLoadDialog("edit_conference")
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	button := dlg.WhichButton(ctxt)
	if button == "cancel" {
		return "redirect", fmt.Sprintf("/comm/%s/conf/%s/manage", comm.Alias, ctxt.GetScratch("currentAlias")), nil
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

	return "redirect", fmt.Sprintf("/comm/%s/conf/%s/manage", comm.Alias, ctxt.GetScratch("currentAlias")), nil
}

/* CreateConferenceForm displays the dialog for creating a new conference.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func CreateConferenceForm(ctxt ui.AmContext) (string, any, error) {
	comm := ctxt.CurrentCommunity()
	if !comm.TestPermission("Community.Create", ctxt.EffectiveLevel()) {
		ctxt.SetRC(http.StatusForbidden)
		return ui.ErrorPage(ctxt, ENOPERM)
	}

	dlg, err := ui.AmLoadDialog("create_conference")
	if err != nil {
		return ui.ErrorPage(ctxt, err)
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
 *     Standard Go error status.
 */
func CreateConference(ctxt ui.AmContext) (string, any, error) {
	comm := ctxt.CurrentCommunity()
	if !comm.TestPermission("Community.Create", ctxt.EffectiveLevel()) {
		ctxt.SetRC(http.StatusForbidden)
		return ui.ErrorPage(ctxt, ENOPERM)
	}

	dlg, err := ui.AmLoadDialog("create_conference")
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	button := dlg.WhichButton(ctxt)
	if button == "cancel" {
		return "redirect", fmt.Sprintf("/comm/%s/conf", comm.Alias), nil
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
	return "redirect", fmt.Sprintf("/comm/%s/conf/%s", comm.Alias, alias), nil
}

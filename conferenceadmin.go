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
	"net/http"

	"git.erbosoft.com/amy/amsterdam/database"
	"git.erbosoft.com/amy/amsterdam/ui"
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

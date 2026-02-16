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
	"git.erbosoft.com/amy/amsterdam/database"
	"git.erbosoft.com/amy/amsterdam/ui"
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

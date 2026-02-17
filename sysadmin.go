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
	"strconv"

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

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
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"git.erbosoft.com/amy/amsterdam/database"
	"git.erbosoft.com/amy/amsterdam/ui"
	"git.erbosoft.com/amy/amsterdam/util"
	"github.com/biter777/countries"
	"github.com/labstack/echo/v4"
)

// ENOJOIN is an error for not being permitted to join a community.
var ENOJOIN *echo.HTTPError = echo.NewHTTPError(http.StatusForbidden, "you are not permitted to join this community")

// ENOUNJOIN is an error for not being permitted to unjoin a community.
var ENOUNJOIN *echo.HTTPError = echo.NewHTTPError(http.StatusForbidden, "you are not permitted to unjoin this community")

/* ShowCommunity renders the community profile display.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func ShowCommunity(ctxt ui.AmContext) (string, any) {
	me := ctxt.CurrentUser()
	prefs, err := me.Prefs(ctxt.Ctx())
	if err != nil {
		return "error", err
	}
	comm := ctxt.CurrentCommunity() // set by middleware
	ci, err := comm.ContactInfo(ctxt.Ctx())
	if err != nil {
		return "error", err
	}
	host, err := comm.Host(ctxt.Ctx())
	if err != nil {
		return "error", err
	}
	var cats []*database.Category
	if !ctxt.GlobalFlags().Get(database.GlobalFlagNoCategories) {
		cats, err = database.AmGetCategoryHierarchy(ctxt.Ctx(), comm.CategoryId)
		if err != nil {
			return "error", err
		}
	}
	var pvtAddr bool
	if ctxt.TestPermission("Global.SeeHiddenContactInfo") {
		pvtAddr = false
	} else {
		pvtAddr = ci.PrivateAddr
	}

	ctxt.VarMap().Set("commName", comm.Name)
	ctxt.VarMap().Set("commAlias", comm.Alias)
	if ci.PhotoURL != nil && *ci.PhotoURL != "" {
		ctxt.VarMap().Set("logoURL", *ci.PhotoURL)
	} else {
		ctxt.VarMap().Set("logoURL", "/img/builtin/default-community.jpg")
	}
	tz := prefs.Location()
	loc := prefs.Localizer()
	ctxt.VarMap().Set("dateCreated", loc.Strftime("%x %X", comm.CreateDate.In(tz)))
	if comm.LastAccess != nil {
		ctxt.VarMap().Set("dateLastAccess", loc.Strftime("%x %X", (*comm.LastAccess).In(tz)))
	}
	if comm.LastUpdate != nil {
		ctxt.VarMap().Set("dateLastUpdate", loc.Strftime("%x %X", (*comm.LastUpdate).In(tz)))
	}
	if !ctxt.IsMember() && ctxt.EffectiveLevel() >= comm.JoinLevel {
		ctxt.VarMap().Set("canJoin", true)
	}
	if ctxt.IsMember() && !me.IsAnon {
		ctxt.VarMap().Set("canInvite", true)
	}
	ctxt.VarMap().Set("public", comm.Public())
	if !ctxt.GlobalFlags().Get(database.GlobalFlagNoCategories) {
		ctxt.VarMap().Set("categories", cats)
	}
	if comm.Synopsis != nil && *comm.Synopsis != "" {
		ctxt.VarMap().Set("description", *comm.Synopsis)
	}
	ctxt.VarMap().Set("hostName", host.Username)
	if ci.Company != nil && *ci.Company != "" {
		ctxt.VarMap().Set("company", *ci.Company)
	}
	if !pvtAddr && ci.Addr1 != nil && *ci.Addr1 != "" {
		ctxt.VarMap().Set("addr1", *ci.Addr1)
	}
	if !pvtAddr && ci.Addr2 != nil && *ci.Addr2 != "" {
		ctxt.VarMap().Set("addr2", *ci.Addr2)
	}
	var b strings.Builder
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
	if ci.Country != nil && *ci.Country != "" {
		country := countries.ByName(*ci.Country)
		ctxt.VarMap().Set("country", country.Emoji()+" "+country.String())
	}
	tag, err := comm.LanguageTag()
	if err == nil && tag != nil {
		ctxt.VarMap().Set("language", util.AmLanguageInLanguage(*tag, *prefs.LanguageTag()))
	}
	if comm.Rules != nil && *comm.Rules != "" {
		ctxt.VarMap().Set("rules", *comm.Rules)
	}
	if ci.URL != nil && *ci.URL != "" {
		ctxt.VarMap().Set("homePage", *ci.URL)
	}

	ctxt.VarMap().Set("amsterdam_pageTitle", "Community Profile: "+comm.Name)
	return "framed", "comprofile.jet"
}

/* JoinCommunity joins a public community, or starts the process of joining a private one.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func JoinCommunity(ctxt ui.AmContext) (string, any) {
	me := ctxt.CurrentUser()
	comm := ctxt.CurrentCommunity() // set by middleware
	mbr, _, _, err := comm.Membership(ctxt.Ctx(), me)
	if err != nil {
		return "error", err
	}
	if mbr {
		// already member, this is a no-op
		return "redirect", fmt.Sprintf("/comm/%s/profile", comm.Alias)
	}
	if comm.TestPermission("Community.Join", me.BaseLevel) {
		if comm.JoinKey != nil && *comm.JoinKey != "" {
			dlg, err := ui.AmLoadDialog("join")
			if err != nil {
				return "error", err
			}
			dlg.SetCommunity(comm)
			dlg.Field("cc").Value = comm.Alias
			return dlg.Render(ctxt)
		}
		// if get here, this is a public community, and we can join
		err = comm.SetMembership(ctxt.Ctx(), me, database.AmDefaultRole("Community.NewUser").Level(), false, me.Uid, ctxt.RemoteIP())
		if err != nil {
			return "error", err
		}
	} else {
		return "error", ENOJOIN
	}
	return "redirect", fmt.Sprintf("/comm/%s/profile", comm.Alias)
}

/* JoinCommunityWithKey joins a private community with a properly specified join key.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func JoinCommunityWithKey(ctxt ui.AmContext) (string, any) {
	me := ctxt.CurrentUser()
	comm := ctxt.CurrentCommunity() // set by middleware
	mbr, _, _, err := comm.Membership(ctxt.Ctx(), me)
	if err != nil {
		return "error", err
	}
	if mbr {
		// already member, this is a no-op
		return "redirect", fmt.Sprintf("/comm/%s/profile", comm.Alias)
	}
	if comm.TestPermission("Community.Join", me.BaseLevel) {
		dlg, err := ui.AmLoadDialog("join")
		if err != nil {
			return "error", err
		}
		dlg.SetCommunity(comm)
		dlg.LoadFromForm(ctxt)
		action := dlg.WhichButton(ctxt)
		if action == "cancel" {
			return "redirect", fmt.Sprintf("/comm/%s/profile", comm.Alias)
		}
		if action == "join_now" {
			key := dlg.Field("key").Value
			dlg.Field("key").Value = "" // clear it in case we redisplay the form
			if key == "" {
				return dlg.RenderError(ctxt, "No join key specified.  Please try again.")
			}
			if comm.JoinKey != nil && key != *comm.JoinKey {
				return dlg.RenderError(ctxt, "The join key does not match the community.  Please try again.")
			}
			err = comm.SetMembership(ctxt.Ctx(), me, database.AmDefaultRole("Community.NewUser").Level(), false, me.Uid, ctxt.RemoteIP())
			if err != nil {
				return dlg.RenderError(ctxt, fmt.Sprintf("Error joining: %v", err))
			}
			return "redirect", fmt.Sprintf("/comm/%s/profile", comm.Alias)
		}
		return dlg.RenderError(ctxt, "Unknown button pressed on join form.")
	}
	return "redirect", fmt.Sprintf("/comm/%s/profile", comm.Alias)
}

/* UnjoinCommunity starts the process of unjoining a community.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func UnjoinCommunity(ctxt ui.AmContext) (string, any) {
	me := ctxt.CurrentUser()
	comm := ctxt.CurrentCommunity() // set by middleware
	mbr, lock, _, err := comm.Membership(ctxt.Ctx(), me)
	if err != nil {
		return "error", err
	}
	if !mbr {
		// not a member, just redirect to profile
		return "redirect", fmt.Sprintf("/comm/%s/profile", comm.Alias)
	}
	if lock {
		return "error", ENOUNJOIN
	}
	ctxt.VarMap().Set("comm", comm)
	ctxt.VarMap().Set("amsterdam_pageTitle", "Unjoin Community")
	return "framed", "unjoin.jet"
}

/* UnjoinCommunityConfirm finishes the process of unjoining a community.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func UnjoinCommunityConfirm(ctxt ui.AmContext) (string, any) {
	me := ctxt.CurrentUser()
	comm := ctxt.CurrentCommunity() // set by middleware
	mbr, lock, _, err := comm.Membership(ctxt.Ctx(), me)
	if err != nil {
		return "error", err
	}
	if !mbr {
		// not a member, just redirect to profile
		return "redirect", fmt.Sprintf("/comm/%s/profile", comm.Alias)
	}
	if lock {
		return "error", ENOUNJOIN
	}
	if ctxt.FormFieldIsSet("cancel") {
		return "redirect", fmt.Sprintf("/comm/%s/profile", comm.Alias)
	}
	if ctxt.FormFieldIsSet("unjoin") {
		err = comm.SetMembership(ctxt.Ctx(), me, 0, false, me.Uid, ctxt.RemoteIP())
		if err != nil {
			return "error", err
		}
		ctxt.ClearCommunityContext()
		return "redirect", fmt.Sprintf("/comm/%s/profile", comm.Alias)
	}
	return "error", EBUTTON
}

/* MemberList lists the members of the community.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func MemberList(ctxt ui.AmContext) (string, any) {
	comm := ctxt.CurrentCommunity() // set by middleware
	ofs := 0
	p := ctxt.Parameter("ofs")
	if p != "" {
		v, err := strconv.Atoi(p)
		if err == nil {
			ofs = v
		}
	}
	ctxt.VarMap().Set("comm", comm)
	ctxt.VarMap().Set("hostUid", *comm.HostUid)
	showHidden := ctxt.TestPermission("Community.ShowHiddenMembers")
	ctxt.VarMap().Set("canExport", showHidden)
	ctxt.VarMap().Set("field", "name")
	ctxt.VarMap().Set("oper", "st")
	ctxt.VarMap().Set("term", "")
	ctxt.VarMap().Set("ofs", ofs)
	ctxt.VarMap().Set("amsterdam_pageTitle", "List Members")
	listMax := int(ctxt.Globals().MaxCommunityMemberPage)
	results, total, err := comm.ListMembers(ctxt.Ctx(), database.ListMembersFieldNone, database.ListMembersOperNone, "", ofs*listMax, listMax, showHidden)
	if err != nil {
		return "error", err
	}
	if total == 0 {
		ctxt.VarMap().Set("headerLine", "Community Members: (None)")
	} else {
		ctxt.VarMap().Set("headerLine", fmt.Sprintf("Community Members: (Displaying %d-%d of %d)",
			ofs*listMax+1, ofs*listMax+len(results), total))
	}
	if len(results) > 0 {
		ctxt.VarMap().Set("resultList", results)
		if ofs > 0 {
			ctxt.VarMap().Set("resultShowPrev", true)
		}
		if ofs*listMax+len(results) < total {
			ctxt.VarMap().Set("resultShowNext", true)
		}
	}
	return "framed", "memberlist.jet"
}

/* MemberSearch searches for members of the community.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func MemberSearch(ctxt ui.AmContext) (string, any) {
	comm := ctxt.CurrentCommunity() // set by middleware
	ofs, _ := ctxt.FormFieldInt("ofs")
	field := ctxt.FormField("field")
	oper := ctxt.FormField("oper")
	term := ctxt.FormField("term")
	ctxt.VarMap().Set("comm", comm)
	ctxt.VarMap().Set("hostUid", comm.HostUid)
	showHidden := ctxt.TestPermission("Community.ShowHiddenMembers")
	ctxt.VarMap().Set("canExport", showHidden)
	ctxt.VarMap().Set("field", field)
	ctxt.VarMap().Set("oper", oper)
	ctxt.VarMap().Set("term", term)
	ctxt.VarMap().Set("ofs", ofs)
	ctxt.VarMap().Set("amsterdam_pageTitle", "Search for Members")
	var iField, iOper int
	switch field {
	case "name":
		iField = database.ListMembersFieldName
	case "descr":
		iField = database.ListMembersFieldDescription
	case "first":
		iField = database.ListMembersFieldFirstName
	case "last":
		iField = database.ListMembersFieldLastName
	default:
		ctxt.VarMap().Set("errorMessage", "invalid parameter to find")
		return "framed", "memberlist.jet"
	}
	switch oper {
	case "st":
		iOper = database.ListMembersOperPrefix
	case "in":
		iOper = database.ListMembersOperSubstring
	case "re":
		iOper = database.ListMembersOperRegex
	default:
		ctxt.VarMap().Set("errorMessage", "invalid parameter to find")
		return "framed", "memberlist.jet"
	}
	listMax := int(ctxt.Globals().MaxCommunityMemberPage)
	results, total, err := comm.ListMembers(ctxt.Ctx(), iField, iOper, term, ofs*listMax, listMax, showHidden)
	if err != nil {
		return "error", err
	}
	if total == 0 {
		ctxt.VarMap().Set("headerLine", "Search Results: (None)")
	} else {
		ctxt.VarMap().Set("headerLine", fmt.Sprintf("Search Results: (Displaying %d-%d of %d)",
			ofs*listMax+1, ofs*listMax+len(results), total))
	}
	if len(results) > 0 {
		ctxt.VarMap().Set("resultList", results)
		if ofs > 0 {
			ctxt.VarMap().Set("resultShowPrev", true)
		}
		if ofs*listMax+len(results) < total {
			ctxt.VarMap().Set("resultShowNext", true)
		}
	}
	return "framed", "memberlist.jet"
}

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
	"net/http"
	"strings"

	"git.erbosoft.com/amy/amsterdam/database"
	"git.erbosoft.com/amy/amsterdam/ui"
	"github.com/biter777/countries"
	"golang.org/x/text/language/display"
)

func ShowCommunity(ctxt ui.AmContext) (string, any, error) {
	me := ctxt.CurrentUser()
	prefs, err := me.Prefs()
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	globals, err := database.AmGlobals()
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	globalFlags, err := globals.Flags()
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	comm, err := database.AmGetCommunityFromParam(ctxt.URLParam("cid"))
	if err != nil {
		ctxt.SetRC(http.StatusNotFound)
		return ui.ErrorPage(ctxt, err)
	}
	member, _, level, err := comm.Membership(me)
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	effectiveLevel := me.BaseLevel
	if member && level > effectiveLevel {
		effectiveLevel = level
	}
	ci, err := comm.ContactInfo()
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	host, err := comm.Host()
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	var cats []*database.Category
	if !globalFlags.Get(database.GlobalFlagNoCategories) {
		cats, err = database.AmGetCategoryHierarchy(comm.CategoryId)
		if err != nil {
			return ui.ErrorPage(ctxt, err)
		}
	}
	var pvtAddr bool
	if database.AmTestPermission("Global.SeeHiddenContactInfo", effectiveLevel) {
		pvtAddr = false
	} else {
		pvtAddr = ci.PrivateAddr
	}

	ctxt.VarMap().Set("commName", comm.Name)
	// TODO: set photo URL
	tz := prefs.Location()
	loc := prefs.Localizer()
	ctxt.VarMap().Set("dateCreated", loc.Strftime("%x %X", comm.CreateDate.In(tz)))
	if comm.LastAccess != nil {
		ctxt.VarMap().Set("dateLastAccess", loc.Strftime("%x %X", (*comm.LastAccess).In(tz)))
	}
	if comm.LastUpdate != nil {
		ctxt.VarMap().Set("dateLastUpdate", loc.Strftime("%x %X", (*comm.LastUpdate).In(tz)))
	}
	if !member && effectiveLevel >= comm.JoinLevel {
		ctxt.VarMap().Set("canJoin", true)
	}
	if member && !me.IsAnon {
		ctxt.VarMap().Set("canInvite", true)
	}
	ctxt.VarMap().Set("public", comm.Public())
	if !globalFlags.Get(database.GlobalFlagNoCategories) {
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
		ctxt.VarMap().Set("country", country.String())
	}
	tag, err := comm.LanguageTag()
	if err == nil && tag != nil {
		ctxt.VarMap().Set("language", display.Languages(*prefs.LanguageTag()).Name(tag))
	}
	if comm.Rules != nil && *comm.Rules != "" {
		ctxt.VarMap().Set("rules", *comm.Rules)
	}
	if ci.URL != nil && *ci.URL != "" {
		ctxt.VarMap().Set("homePage", *ci.URL)
	}

	ctxt.VarMap().Set("amsterdam_pageTitle", "Community Profile: "+comm.Name)
	return "framed_template", "comprofile.jet", nil
}

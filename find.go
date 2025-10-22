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
	"strconv"

	"git.erbosoft.com/amy/amsterdam/database"
	"git.erbosoft.com/amy/amsterdam/ui"
)

// loadCategoryInformation loads the current category information to the context.
func loadCategoryInformation(ctxt ui.AmContext, offset int) error {
	if ctxt.GlobalFlags().Get(database.GlobalFlagNoCategories) {
		return nil
	}
	u := ctxt.CurrentUser()
	catid := int32(-1)
	p := ctxt.Parameter("catid")
	if p != "" {
		v, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		catid = int32(v)
	} else if ctxt.IsSession("find.catid") {
		x := ctxt.GetSession("find.catid")
		if xx, ok := x.(int32); ok {
			catid = xx
		}
	}
	if catid > -1 {
		cat, err := database.AmGetCategory(catid) // this step also resolves symlinks
		if err != nil {
			return err
		}
		catid = cat.CatId
	}
	ctxt.SetSession("find.catid", catid)
	ctxt.VarMap().Set("catid", catid)
	showHidden := database.AmTestPermission("Global.ShowHiddenCategories", u.BaseLevel)
	ctxt.VarMap().Set("showHiddenCat", showHidden)
	hier, err := database.AmGetCategoryHierarchy(catid)
	if err != nil {
		return err
	}
	ctxt.VarMap().Set("catHierarchy", hier)
	subs, err := database.AmGetSubCategories(catid)
	if err != nil {
		return err
	}
	ctxt.VarMap().Set("catSubs", subs)
	ctxt.VarMap().Set("displayCats", true)

	if catid > -1 {
		// search for communities in this category
		listMax := int(ctxt.Globals().MaxSearchPage)
		commList, numComm, err := database.AmGetCommunitiesForCategory(catid, offset*listMax, listMax, showHidden)
		if err != nil {
			return err
		}
		if len(commList) == 0 {
			ctxt.VarMap().Set("resultHeader", "Communities in Category (None)")
		} else {
			ctxt.VarMap().Set("resultHeader", fmt.Sprintf("Communities in Category (Displaying %d-%d of %d)",
				offset*listMax+1, offset*listMax+len(commList), numComm))
			ctxt.VarMap().Set("resultList", commList)
			ctxt.VarMap().Set("resultFromDirectory", true)
			if offset > 0 {
				ctxt.VarMap().Set("resultShowPrev", true)
			}
			if offset*listMax+len(commList) < numComm {
				ctxt.VarMap().Set("resultShowNext", true)
			}
		}
	}
	return nil
}

/* FindPage renders the Amsterdam "Find" page.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func FindPage(ctxt ui.AmContext) (string, any, error) {
	mode := ""
	p := ctxt.Parameter("mode")
	if p != "" {
		mode = p
	} else if ctxt.IsSession("find.mode") {
		x := ctxt.GetSession("find.mode")
		if xx, ok := x.(string); ok {
			mode = xx
		}
	}
	if mode == "" {
		mode = "COM"
	}
	ofs := 0
	p = ctxt.Parameter("ofs")
	if p != "" {
		v, err := strconv.Atoi(p)
		if err == nil {
			ofs = v
		}
	}
	if !ctxt.GlobalFlags().Get(database.GlobalFlagNoCategories) {
		ctxt.VarMap().Set("catIsPresent", true)
	}
	ctxt.SetSession("find.mode", mode)
	ctxt.VarMap().Set("mode", mode)
	ctxt.VarMap().Set("ofs", ofs)
	switch mode {
	case "COM":
		ctxt.VarMap().Set("field", "name")
		ctxt.VarMap().Set("oper", "st")
		ctxt.VarMap().Set("term", "")
		err := loadCategoryInformation(ctxt, ofs)
		if err != nil {
			return ui.ErrorPage(ctxt, err)
		}
	case "USR":
		ctxt.VarMap().Set("field", "name")
		ctxt.VarMap().Set("oper", "st")
		ctxt.VarMap().Set("term", "")
	case "CAT":
		ctxt.VarMap().Set("field", "name")
		ctxt.VarMap().Set("oper", "st")
		ctxt.VarMap().Set("term", "")
	case "PST":
		ctxt.VarMap().Set("field", "name")
		ctxt.VarMap().Set("oper", "in")
		ctxt.VarMap().Set("term", "")
	}

	ctxt.VarMap().Set("amsterdam_pageTitle", "Find")
	ctxt.SetLeftMenu("top")
	return "framed_template", "find.jet", nil
}

/* Find performs the "find" operation.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func Find(ctxt ui.AmContext) (string, any, error) {
	if !ctxt.GlobalFlags().Get(database.GlobalFlagNoCategories) {
		ctxt.VarMap().Set("catIsPresent", true)
	}
	mode := ctxt.FormField("mode")
	ctxt.VarMap().Set("mode", mode)
	field := ctxt.FormField("field")
	ctxt.VarMap().Set("field", field)
	oper := ctxt.FormField("oper")
	ctxt.VarMap().Set("oper", oper)
	term := ctxt.FormField("term")
	ctxt.VarMap().Set("term", term)
	ctxt.VarMap().Set("amsterdam_pageTitle", "Find")
	ctxt.SetLeftMenu("top")
	ofs, _ := ctxt.FormFieldInt("ofs")
	if ctxt.FormFieldIsSet("search") {
		ofs = 0
	} else if ctxt.FormFieldIsSet("prev") {
		ofs -= 1
	} else if ctxt.FormFieldIsSet("next") {
		ofs += 1
	}
	ctxt.VarMap().Set("ofs", ofs)
	listMax := int(ctxt.Globals().MaxSearchPage)
	var numResults, total int
	var err error
	switch mode {
	case "COM":
		var iField, iOper int
		switch field {
		case "name":
			iField = database.SearchCommFieldName
		case "synopsis":
			iField = database.SearchCommFieldSynopsis
		default:
			ctxt.VarMap().Set("errorMessage", "invalid parameter to find")
			return "framed_template", "find.jet", nil
		}
		switch oper {
		case "st":
			iOper = database.SearchCommOperPrefix
		case "in":
			iOper = database.SearchCommOperSubstring
		case "re":
			iOper = database.SearchCommOperRegex
		default:
			ctxt.VarMap().Set("errorMessage", "invalid parameter to find")
			return "framed_template", "find.jet", nil
		}
		var clist []*database.Community
		clist, total, err = database.AmSearchCommunities(iField, iOper, term, ofs*listMax, listMax,
			ctxt.TestPermission("Global.SearchHiddenCommunities"))
		if err == nil {
			if clist == nil {
				numResults = 0
			} else {
				numResults = len(clist)
				ctxt.VarMap().Set("resultList", clist)
			}
		}
	case "USR":
		var iField, iOper int
		switch field {
		case "name":
			iField = database.SearchUserFieldName
		case "descr":
			iField = database.SearchUserFieldDescription
		case "first":
			iField = database.SearchUserFieldFirstName
		case "last":
			iField = database.SearchUserFieldLastName
		default:
			ctxt.VarMap().Set("errorMessage", "invalid parameter to find")
			return "framed_template", "find.jet", nil
		}
		switch oper {
		case "st":
			iOper = database.SearchUserOperPrefix
		case "in":
			iOper = database.SearchUserOperSubstring
		case "re":
			iOper = database.SearchUserOperRegex
		default:
			ctxt.VarMap().Set("errorMessage", "invalid parameter to find")
			return "framed_template", "find.jet", nil
		}
		var ulist []*database.User
		ulist, total, err = database.AmSearchUsers(iField, iOper, term, ofs*listMax, listMax)
		if err == nil {
			if ulist == nil {
				numResults = 0
			} else {
				numResults = len(ulist)
				ctxt.VarMap().Set("resultList", ulist)
			}
		}
	case "CAT":
		// TODO
	case "PST":
		// TODO
	}
	if err != nil {
		ctxt.VarMap().Set("errorMessage", err.Error())
		return "framed_template", "find.jet", nil
	}
	if numResults == 0 {
		ctxt.VarMap().Set("resultHeader", "Search Results: (None)")
	} else {
		ctxt.VarMap().Set("resultHeader", fmt.Sprintf("Search Results: Displaying %d-%d of %d",
			ofs*listMax+1, ofs*listMax+numResults, total))
		if ofs > 0 {
			ctxt.VarMap().Set("resultShowPrev", true)
		}
		if ofs*listMax+numResults < total {
			ctxt.VarMap().Set("resultShowNext", true)
		}
	}
	return "framed_template", "find.jet", nil
}

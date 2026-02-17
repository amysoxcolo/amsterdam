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

// SearchUserFieldMap maps field names to search field indexes.
var SearchUserFieldMap = map[string]int{
	"name":  database.SearchUserFieldName,
	"descr": database.SearchUserFieldDescription,
	"first": database.SearchUserFieldFirstName,
	"last":  database.SearchUserFieldLastName,
}

// SearchUserOperMap maps operator names to search operator indices.
var SearchUserOperMap = map[string]int{
	"st": database.SearchUserOperPrefix,
	"in": database.SearchUserOperSubstring,
	"re": database.SearchUserOperRegex,
}

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
		cat, err := database.AmGetCategory(ctxt.Ctx(), catid) // this step also resolves symlinks
		if err != nil {
			return err
		}
		catid = cat.CatId
	}
	ctxt.SetSession("find.catid", catid)
	ctxt.VarMap().Set("catid", catid)
	showHidden := database.AmTestPermission("Global.ShowHiddenCategories", u.BaseLevel)
	ctxt.VarMap().Set("showHiddenCat", showHidden)
	hier, err := database.AmGetCategoryHierarchy(ctxt.Ctx(), catid)
	if err != nil {
		return err
	}
	ctxt.VarMap().Set("catHierarchy", hier)
	subs, err := database.AmGetSubCategories(ctxt.Ctx(), catid)
	if err != nil {
		return err
	}
	ctxt.VarMap().Set("catSubs", subs)
	ctxt.VarMap().Set("displayCats", true)

	if catid > -1 {
		// search for communities in this category
		listMax := int(ctxt.Globals().MaxSearchPage)
		commList, numComm, err := database.AmGetCommunitiesForCategory(ctxt.Ctx(), catid, offset*listMax, listMax, showHidden)
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
 */
func FindPage(ctxt ui.AmContext) (string, any) {
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
			return "error", err
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

	ctxt.SetFrameTitle("Find")
	ctxt.SetLeftMenu("top")
	return "framed", "find.jet"
}

/* Find performs the "find" operation.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func Find(ctxt ui.AmContext) (string, any) {
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
	ctxt.SetFrameTitle("Find")
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
			return "framed", "find.jet"
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
			return "framed", "find.jet"
		}
		var clist []*database.Community
		clist, total, err = database.AmSearchCommunities(ctxt.Ctx(), iField, iOper, term, ofs*listMax, listMax,
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
		var ok bool
		if iField, ok = SearchUserFieldMap[field]; !ok {
			ctxt.VarMap().Set("errorMessage", "invalid parameter to find")
			return "framed", "find.jet"
		}
		if iOper, ok = SearchUserOperMap[oper]; !ok {
			ctxt.VarMap().Set("errorMessage", "invalid parameter to find")
			return "framed", "find.jet"
		}
		var ulist []*database.User
		ulist, total, err = database.AmSearchUsers(ctxt.Ctx(), iField, iOper, term, ofs*listMax, listMax)
		if err == nil {
			if ulist == nil {
				numResults = 0
			} else {
				numResults = len(ulist)
				ctxt.VarMap().Set("resultList", ulist)
			}
		}
	case "CAT":
		listMax = 20
		var iOper int
		switch oper {
		case "st":
			iOper = database.SearchCatOperPrefix
		case "in":
			iOper = database.SearchCatOperSubstring
		case "re":
			iOper = database.SearchCatOperRegex
		default:
			ctxt.VarMap().Set("errorMessage", "invalid parameter to find")
			return "framed", "find.jet"
		}
		var catlist []*database.Category
		catlist, total, err = database.AmSearchCategories(ctxt.Ctx(), iOper, term, ofs*listMax, listMax,
			ctxt.TestPermission("Global.ShowHiddenCategories"), ctxt.TestPermission("Global.SearchHiddenCategories"))
		if err == nil {
			if catlist == nil {
				numResults = 0
			} else {
				numResults = len(catlist)
				ctxt.VarMap().Set("resultList", catlist)
			}
		}
	case "PST":
		var postlist []database.PostSearchResult
		postlist, total, err = database.AmSearchPosts(ctxt.Ctx(), term, ctxt.CurrentUser(), ofs*listMax, listMax)
		if err == nil {
			numResults = len(postlist)
			ctxt.VarMap().Set("resultList", postlist)
		}
	}
	if err != nil {
		ctxt.VarMap().Set("errorMessage", err.Error())
		return "framed", "find.jet"
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
	return "framed", "find.jet"
}

// commonFindGetBackend is the common "back end" function for Find Posts in Community/Conference/Topic.
func commonFindGetBackend(ctxt ui.AmContext) (string, any) {
	ofs := 0
	p := ctxt.Parameter("ofs")
	if p != "" {
		v, err := strconv.Atoi(p)
		if err == nil {
			ofs = v
		}
	}
	ctxt.VarMap().Set("ofs", ofs)
	ctxt.VarMap().Set("term", "")
	ctxt.SetFrameTitle("Find Posts")
	return "framed", "find_posts.jet"
}

/* FindPostsPageCommunity renders the page for finding posts in a community.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func FindPostsPageCommunity(ctxt ui.AmContext) (string, any) {
	comm := ctxt.CurrentCommunity()
	ctxt.VarMap().Set("scope", "community")
	ctxt.VarMap().Set("entityName", comm.Name)
	ctxt.VarMap().Set("backlink", fmt.Sprintf("/comm/%s/conf", comm.Alias))
	ctxt.VarMap().Set("postlink", fmt.Sprintf("/comm/%s/find", comm.Alias))
	return commonFindGetBackend(ctxt)
}

/* FindPostsPageConference renders the page for finding posts in a conference.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func FindPostsPageConference(ctxt ui.AmContext) (string, any) {
	comm := ctxt.CurrentCommunity()
	conf := ctxt.GetScratch("currentConference").(*database.Conference)
	ctxt.VarMap().Set("scope", "conference")
	ctxt.VarMap().Set("entityName", conf.Name)
	ctxt.VarMap().Set("backlink", fmt.Sprintf("/comm/%s/conf/%s", comm.Alias, ctxt.GetScratch("currentAlias")))
	ctxt.VarMap().Set("postlink", fmt.Sprintf("/comm/%s/conf/%s/find", comm.Alias, ctxt.GetScratch("currentAlias")))
	return commonFindGetBackend(ctxt)
}

/* FindPostsPageTopic renders the page for finding posts in a topic.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func FindPostsPageTopic(ctxt ui.AmContext) (string, any) {
	comm := ctxt.CurrentCommunity()
	topic := ctxt.GetScratch("currentTopic").(*database.Topic)
	ctxt.VarMap().Set("scope", "topic")
	ctxt.VarMap().Set("entityName", topic.Name)
	ctxt.VarMap().Set("backlink", fmt.Sprintf("/comm/%s/conf/%s/r/%d", comm.Alias, ctxt.GetScratch("currentAlias"), topic.Number))
	ctxt.VarMap().Set("postlink", fmt.Sprintf("/comm/%s/conf/%s/op/%d/find", comm.Alias, ctxt.GetScratch("currentAlias"), topic.Number))
	return commonFindGetBackend(ctxt)
}

// commonFindPostBackend is the common "back end" function for Find Posts in Community/Conference/Topic.
func commonFindPostBackend(ctxt ui.AmContext, comm *database.Community, conf *database.Conference, topic *database.Topic) (string, any) {
	term := ctxt.FormField("term")
	ctxt.VarMap().Set("term", term)
	ctxt.SetFrameTitle("Find Posts")
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
	var numResults int
	postlist, total, err := database.AmSearchPosts(ctxt.Ctx(), term, ctxt.CurrentUser(), ofs*listMax, listMax, comm, conf, topic)
	if err == nil {
		numResults = len(postlist)
		ctxt.VarMap().Set("resultList", postlist)
	} else {
		ctxt.VarMap().Set("errorMessage", err.Error())
		return "framed", "find_posts.jet"
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
	return "framed", "find_posts.jet"
}

/* FindPostsCommunity finds posts in a community.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func FindPostsCommunity(ctxt ui.AmContext) (string, any) {
	comm := ctxt.CurrentCommunity()
	ctxt.VarMap().Set("scope", "community")
	ctxt.VarMap().Set("entityName", comm.Name)
	ctxt.VarMap().Set("backlink", fmt.Sprintf("/comm/%s/conf", comm.Alias))
	ctxt.VarMap().Set("postlink", fmt.Sprintf("/comm/%s/find", comm.Alias))
	return commonFindPostBackend(ctxt, comm, nil, nil)
}

/* FindPostsConference finds posts in a conference.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func FindPostsConference(ctxt ui.AmContext) (string, any) {
	comm := ctxt.CurrentCommunity()
	conf := ctxt.GetScratch("currentConference").(*database.Conference)
	ctxt.VarMap().Set("scope", "conference")
	ctxt.VarMap().Set("entityName", conf.Name)
	ctxt.VarMap().Set("backlink", fmt.Sprintf("/comm/%s/conf/%s", comm.Alias, ctxt.GetScratch("currentAlias")))
	ctxt.VarMap().Set("postlink", fmt.Sprintf("/comm/%s/conf/%s/find", comm.Alias, ctxt.GetScratch("currentAlias")))
	return commonFindPostBackend(ctxt, comm, conf, nil)
}

/* FindPostsTopic finds posts in a topic.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func FindPostsTopic(ctxt ui.AmContext) (string, any) {
	comm := ctxt.CurrentCommunity()
	conf := ctxt.GetScratch("currentConference").(*database.Conference)
	topic := ctxt.GetScratch("currentTopic").(*database.Topic)
	ctxt.VarMap().Set("scope", "topic")
	ctxt.VarMap().Set("entityName", topic.Name)
	ctxt.VarMap().Set("backlink", fmt.Sprintf("/comm/%s/conf/%s/r/%d", comm.Alias, ctxt.GetScratch("currentAlias"), topic.Number))
	ctxt.VarMap().Set("postlink", fmt.Sprintf("/comm/%s/conf/%s/op/%d/find", comm.Alias, ctxt.GetScratch("currentAlias"), topic.Number))
	return commonFindPostBackend(ctxt, comm, conf, topic)
}

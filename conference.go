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

// conferencesPrequel consolidates some of the basic conference checks into one function.
func conferencesPrequel(ctxt ui.AmContext) (string, any, error) {
	err := ctxt.SetCommunityContext(ctxt.URLParam("cid"))
	if err != nil {
		ctxt.SetRC(http.StatusNotFound)
		return ui.ErrorPage(ctxt, err)
	}
	comm := ctxt.CurrentCommunity()
	b, err := database.AmTestService(comm, "Conference")
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	if !b {
		ctxt.SetRC(http.StatusNotFound)
		return ui.ErrorPage(ctxt, errors.New("this community does not use conferencing services"))
	}
	if comm.MembersOnly && !ctxt.IsMember() && !ctxt.TestPermission("Community.NoJoinRequired") {
		ctxt.SetRC(http.StatusForbidden)
		return ui.ErrorPage(ctxt, errors.New("you are not a member of this community"))
	}
	if !comm.TestPermission("Community.Read", ctxt.EffectiveLevel()) {
		ctxt.SetRC(http.StatusForbidden)
		return ui.ErrorPage(ctxt, errors.New("you are not authorized access to conferences"))
	}
	return "", nil, nil
}

func singleConferencePrequel(ctxt ui.AmContext) (string, any, error) {
	cmd, arg, err := conferencesPrequel(ctxt)
	if cmd != "" {
		return cmd, arg, err
	}
	var conf *database.Conference
	conf, err = database.AmGetConferenceByAliasInCommunity(ctxt.CurrentCommunity().Id, ctxt.URLParam("confid"))
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	m, lvl, err := conf.Membership(ctxt.CurrentUser())
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	myLevel := ctxt.EffectiveLevel()
	if m && lvl > myLevel {
		myLevel = lvl
	}
	ctxt.SetScratch("currentConference", conf)
	ctxt.SetScratch("levelInConference", myLevel)
	return "", nil, nil
}

/* Conferences displayes the list of conferences in a community.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func Conferences(ctxt ui.AmContext) (string, any, error) {
	cmd, arg, err := conferencesPrequel(ctxt)
	if cmd != "" {
		return cmd, arg, err
	}
	comm := ctxt.CurrentCommunity()
	ctxt.VarMap().Set("commName", comm.Name)
	ctxt.VarMap().Set("commAlias", comm.Alias)
	ctxt.VarMap().Set("amsterdam_pageTitle", "Conference Listing: "+comm.Name)
	clist, err := database.AmGetCommunityConferences(comm.Id,
		comm.TestPermission("Community.ShowHiddenObjects", ctxt.EffectiveLevel()))
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	ctxt.VarMap().Set("conferences", clist)
	return "framed_template", "conflist.jet", err
}

/* Topics displays the list of topics in a conference.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func Topics(ctxt ui.AmContext) (string, any, error) {
	cmd, arg, err := singleConferencePrequel(ctxt)
	if cmd != "" {
		return cmd, arg, err
	}
	prefs, err := ctxt.CurrentUser().Prefs()
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	comm := ctxt.CurrentCommunity()
	conf := ctxt.GetScratch("currentConference").(*database.Conference)
	myLevel := ctxt.GetScratch("levelInConference").(uint16)
	if !conf.TestPermission("Conference.Read", myLevel) {
		ctxt.SetRC(http.StatusForbidden)
		return ui.ErrorPage(ctxt, errors.New("you are not permitted to read this conference"))
	}

	// Get view and sort parameters from query, session, or defaults. Store to session.
	trustSessionValues := false
	if ctxt.IsSession("topic.conf") {
		v := ctxt.GetSession("topic.conf").(int32)
		if v == conf.ConfId {
			trustSessionValues = true
		} else {
			ctxt.SetSession("topic.conf", conf.ConfId)
		}
	}
	view := database.TopicViewActive
	if trustSessionValues && ctxt.IsSession("topic.view") {
		view = ctxt.GetSession("topic.view").(int)
	}
	view = ctxt.QueryParamInt("view", view)
	ctxt.SetSession("topic.view", view)
	sort := database.TopicSortNumber
	if trustSessionValues && ctxt.IsSession("topic.sort") {
		sort = ctxt.GetSession("topic.sort").(int)
	}
	sort = ctxt.QueryParamInt("sort", sort)
	ctxt.SetSession("topic.sort", sort)

	topics, err := database.AmListTopics(conf.ConfId, ctxt.CurrentUserId(), view, sort, false)
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}

	tz := prefs.Location()
	loc := prefs.Localizer()
	fdate := make([]string, len(topics))
	for i, t := range topics {
		fdate[i] = loc.Strftime("%x %X", t.LastUpdate.In(tz))
	}

	ctxt.VarMap().Set("conferenceName", conf.Name)
	ctxt.VarMap().Set("urlBack", fmt.Sprintf("/comm/%s/conf", comm.Alias))
	ctxt.VarMap().Set("urlStem", fmt.Sprintf("/comm/%s/conf/%s", comm.Alias, ctxt.URLParam("confid")))
	ctxt.VarMap().Set("permalink", "TODO")
	ctxt.VarMap().Set("view", view)
	ctxt.VarMap().Set("sort", sort)
	ctxt.VarMap().Set("topics", topics)
	ctxt.VarMap().Set("formattedDate", fdate)
	ctxt.VarMap().Set("amsterdam_pageTitle", "Topics in "+conf.Name)
	return "framed_template", "topiclist.jet", nil
}

/* NewTopicForm displays the form for creating a new topic.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func NewTopicForm(ctxt ui.AmContext) (string, any, error) {
	cmd, arg, err := singleConferencePrequel(ctxt)
	if cmd != "" {
		return cmd, arg, err
	}
	comm := ctxt.CurrentCommunity()
	conf := ctxt.GetScratch("currentConference").(*database.Conference)
	ci, err := ctxt.CurrentUser().ContactInfo()
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	myLevel := ctxt.GetScratch("levelInConference").(uint16)
	if !conf.TestPermission("Conference.Create", myLevel) {
		ctxt.SetRC(http.StatusForbidden)
		return ui.ErrorPage(ctxt, errors.New("you are not permitted to read this conference"))
	}
	ctxt.VarMap().Set("conferenceName", conf.Name)
	ctxt.VarMap().Set("urlStem", fmt.Sprintf("/comm/%s/conf/%s", comm.Alias, ctxt.URLParam("confid")))
	ctxt.VarMap().Set("topicName", "")
	ctxt.VarMap().Set("pseud", ci.FullName(false))
	ctxt.VarMap().Set("pb", "")
	ctxt.VarMap().Set("amsterdam_pageTitle", "Create New Topic")
	return "framed_template", "new_topic.jet", nil
}

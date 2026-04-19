/*
 * Amsterdam Web Communities System
 * Copyright (c) 2025-2026 Erbosoft Metaverse Design Solutions, All Rights Reserved
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * SPDX-License-Identifier: MPL-2.0
 */
// Package main contains the high-level Amsterdam logic.
package main

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"reflect"
	"slices"
	"strings"

	"git.erbosoft.com/amy/amsterdam/config"
	"git.erbosoft.com/amy/amsterdam/database"
	"git.erbosoft.com/amy/amsterdam/ui"
	"github.com/CloudyKit/jet/v6"
	"github.com/labstack/echo/v4"
	log "github.com/sirupsen/logrus"
)

/*----------------------------------------------------------------------------
 * Sidebox rendering
 *----------------------------------------------------------------------------
 */

// SideboxRendering is a wrapper interface used to handle rendering a sidebox's variables.
type SideboxRendering interface {
	SetVar(string, any)
}

// SideboxRenderFunc "renders" a sidebox by outputing variables through an adapter.
type SideboxRenderFunc func(context.Context, *database.User, *DisplaySidebox, *string, SideboxRendering) error

// DisplaySidebox is the structure used to display a sidebox.
type DisplaySidebox struct {
	BoxId        int32             // box ID
	Title        string            // title to display
	TitleAnon    string            // title to display if user is anon
	TemplateName string            // name of template to render
	Renderer     SideboxRenderFunc // rendering function
}

// renderSBCommunities renders the Communities sidebox.
func renderSBCommunities(ctx context.Context, u *database.User, sb *DisplaySidebox, param *string, rx SideboxRendering) error {
	g, err := database.AmGlobals(ctx)
	if err != nil {
		return err
	}
	l, err := database.AmGetCommunitiesForUser(ctx, u.Uid)
	if err != nil {
		return err
	}
	rx.SetVar("communities", l)
	isAdmin := make([]bool, len(l))
	for i, c := range l {
		isAdmin[i] = false
		level, err := database.AmGetCommunityAccessLevel(ctx, u.Uid, c.Id)
		if err == nil && database.AmTestPermission("Community.ShowAdmin", level) {
			isAdmin[i] = true
		}
	}
	rx.SetVar("isAdmin", isAdmin)
	rx.SetVar("canManage", !(u.IsAnon))
	rx.SetVar("canCreate", !(u.IsAnon) && u.BaseLevel >= uint16(g.CommunityCreateLevel))
	return nil
}

// renderSBConferences renders the Conferences sidebox.
func renderSBConferences(ctx context.Context, u *database.User, sb *DisplaySidebox, param *string, rx SideboxRendering) error {
	hl, err := database.AmGetConferenceHotlist(ctx, u)
	if err != nil {
		return err
	}
	comm := make([]*database.Community, len(hl))
	conf := make([]*database.Conference, len(hl))
	alias := make([]string, len(hl))
	newFlag := make([]bool, len(hl))
	for i, h := range hl {
		if comm[i], err = h.Community(ctx); err != nil {
			return err
		}
		if conf[i], err = h.Conference(ctx); err != nil {
			return err
		}
		var a []string
		if a, err = conf[i].Aliases(ctx, comm[i].Id); err != nil {
			return err
		}
		alias[i] = a[0]
		newFlag[i] = false
		if !u.IsAnon {
			nnew, err := conf[i].UnreadMessages(ctx, u)
			if err == nil {
				newFlag[i] = (nnew > 0)
			}
		}
	}
	rx.SetVar("comm", comm)
	rx.SetVar("conf", conf)
	rx.SetVar("alias", alias)
	rx.SetVar("newFlag", newFlag)
	rx.SetVar("canManage", !(u.IsAnon))
	return nil
}

// renderSBOnlineUsers renders the Online Users sidebox.
func renderSBOnlineUsers(ctx context.Context, u *database.User, sb *DisplaySidebox, param *string, rx SideboxRendering) error {
	anons, users, maxUsers := ui.AmSessions()
	rx.SetVar("total", len(users)+anons)
	rx.SetVar("maxUsers", maxUsers)
	rx.SetVar("anons", anons)
	rx.SetVar("users", users)
	return nil
}

// sideboxRegistry contains a registry of all known sideboxes.
var sideboxRegistry map[int32]*DisplaySidebox

// init sets up the sidebox registry.
func init() {
	sideboxRegistry = make(map[int32]*DisplaySidebox)
	sb1 := DisplaySidebox{
		BoxId:        database.SideboxIDCommunities,
		Title:        "Your Communities",
		TitleAnon:    "Featured Communities",
		TemplateName: "sb_comm.jet",
		Renderer:     renderSBCommunities,
	}
	sideboxRegistry[database.SideboxIDCommunities] = &sb1
	sb2 := DisplaySidebox{
		BoxId:        database.SideboxIDConferences,
		Title:        "Your Conference Hotlist",
		TitleAnon:    "Featured Conferences",
		TemplateName: "sb_conf.jet",
		Renderer:     renderSBConferences,
	}
	sideboxRegistry[database.SideboxIDConferences] = &sb2
	sb3 := DisplaySidebox{
		BoxId:        database.SideboxIDOnlineUsers,
		Title:        "Users Online",
		TitleAnon:    "Users Online",
		TemplateName: "sb_online.jet",
		Renderer:     renderSBOnlineUsers,
	}
	sideboxRegistry[database.SideboxIDOnlineUsers] = &sb3
}

// sbRender is a context used for controlling adding variables for sideboxes.
type sbRender struct {
	ctxt ui.AmContext // the UI context
	id   int32        // ID of sidebox being rendered
}

// SetVar sets a sidebox rendering value into the context.
func (rx *sbRender) SetVar(name string, value any) {
	rx.ctxt.VarMap().Set(fmt.Sprintf("sb%d_%s", rx.id, name), value)
}

// templateGetTopic returns the pointer to the topic.
func templateGetTopic(args jet.Arguments) reflect.Value {
	post := args.Get(0).Convert(reflect.TypeFor[*database.PostHeader]()).Interface().(*database.PostHeader)
	ctxt := args.Get(1).Convert(reflect.TypeFor[ui.AmContext]()).Interface().(ui.AmContext)
	topic, _ := database.AmGetTopic(ctxt.Ctx(), post.TopicId)
	return reflect.ValueOf(topic)
}

// templateTopicLink returns the link string for the given topic.
func templateTopicLink(args jet.Arguments) reflect.Value {
	comm := args.Get(0).Convert(reflect.TypeFor[*database.Community]()).Interface().(*database.Community)
	topic := args.Get(1).Convert(reflect.TypeFor[*database.Topic]()).Interface().(*database.Topic)
	ctxt := args.Get(2).Convert(reflect.TypeFor[ui.AmContext]()).Interface().(ui.AmContext)
	link, _ := topic.Link(ctxt.Ctx(), comm.Id, "global")
	return reflect.ValueOf(link)
}

/* TopPage renders the "top level" Amsterdam page (the "home page").
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func TopPage(ctxt ui.AmContext) (string, any) {
	// Set the page title.
	ctxt.SetFrameTitle("My Front Page")

	// Retrieve the published posts.
	hdrs, comms, err := database.AmGetPublishedPosts(ctxt.Ctx())
	if err != nil {
		return "error", err
	}

	ctxt.VarMap().Set("posts", hdrs)
	ctxt.VarMap().Set("comms", comms)
	ctxt.VarMap().SetFunc("post_getText", templatePostText)
	ctxt.VarMap().SetFunc("post_getUserName", templateExtractUserName)
	ctxt.VarMap().SetFunc("post_topic", templateGetTopic)
	ctxt.VarMap().SetFunc("post_topicLink", templateTopicLink)

	// Retrieve the sideboxes and create the data to be presented.
	sboxes, err := database.AmGetSideboxes(ctxt.Ctx(), ctxt.CurrentUserId())
	if err != nil {
		return "error", err
	}
	disp := make([]*DisplaySidebox, 0, len(sboxes))
	rx := sbRender{ctxt: ctxt, id: 0}
	for _, sb := range sboxes {
		if dsb, ok := sideboxRegistry[sb.Boxid]; ok {
			rx.id = sb.Boxid
			err := dsb.Renderer(ctxt.Ctx(), ctxt.CurrentUser(), dsb, sb.Param, &rx)
			if err != nil {
				return "error", err
			}
			disp = append(disp, dsb)
		} else {
			log.Errorf("TopPage: unknown sidebox ID %d", sb.Boxid)
		}
	}
	ctxt.VarMap().Set("sideboxes", disp)

	// Final data set.
	ctxt.SetLeftMenu("top")
	if config.GlobalConfig.Site.TopRefresh > 0 {
		ctxt.AddFrameMetadata(ui.FrameMetaHttpEquiv, "refresh", fmt.Sprintf("%d", config.GlobalConfig.Site.TopRefresh))
	}
	return "framed", "top.jet"
}

/* AboutPage renders the "About Amsterdam" page.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func AboutPage(ctxt ui.AmContext) (string, any) {
	// Set the database version.
	g, err := database.AmGlobals(ctxt.Ctx())
	if err != nil {
		return "error", err
	}
	ctxt.VarMap().Set("dbVersion", g.Version)

	// Set the page title.
	ctxt.SetFrameTitle("About Amsterdam")
	return "framed", "about.jet"
}

/* PolicyPage renders the policy page.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func PolicyPage(ctxt ui.AmContext) (string, any) {
	title, body, err := ui.AmLoadHTMLResource(config.GlobalConfig.Site.PolicyResource)
	if err != nil {
		return "error", err
	}

	ctxt.VarMap().Set("title", title)
	ctxt.VarMap().Set("body", body)
	ctxt.SetFrameTitle(title)
	return "framed", "policy.jet"
}

/* JumpToShortcut resolves "/go" links by redirecting them to the appropriate page.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func JumpToShortcut(ctxt ui.AmContext) (string, any) {
	link, err := database.AmDecodePostLink(ctxt.URLParam("postlink"))
	if err != nil {
		return "error", echo.NewHTTPError(http.StatusNotFound, fmt.Sprintf("not found: %s", ctxt.URLParam("postlink"))).SetInternal(err)
	}
	scope, target := link.Classify()
	if scope != "global" {
		return "error", echo.NewHTTPError(http.StatusNotFound, fmt.Sprintf("not found: %s", ctxt.URLParam("postlink")))
	}
	if err = link.VerifyNames(ctxt.Ctx()); err != nil {
		return "error", echo.NewHTTPError(http.StatusNotFound, fmt.Sprintf("not found: %s", ctxt.URLParam("postlink"))).SetInternal(err)
	}
	targetURL := ""
	switch target {
	case "community":
		targetURL = fmt.Sprintf("/comm/%s", link.Community)
	case "conference":
		targetURL = fmt.Sprintf("/comm/%s/conf/%s", link.Community, link.Conference)
	case "topic":
		targetURL = fmt.Sprintf("/comm/%s/conf/%s/r/%d", link.Community, link.Conference, link.Topic)
	case "post", "postrange", "postopenrange":
		targetURL = fmt.Sprintf("/comm/%s/conf/%s/r/%d?r=%d,%d", link.Community, link.Conference, link.Topic, link.FirstPost, link.LastPost)
	default:
		return "error", fmt.Sprintf("invalid target '%s' for link: %s", target, ctxt.URLParam("postlink"))
	}
	return "redirect", targetURL
}

/* ManageSideboxes displays the "sidebox management" page.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func ManageSideboxes(ctxt ui.AmContext) (string, any) {
	if ctxt.CurrentUser().IsAnon {
		return "error", ELOGIN
	}

	// Retrieve the sideboxes from the database.
	sboxes, err := database.AmGetSideboxes(ctxt.Ctx(), ctxt.CurrentUserId())
	if err != nil {
		return "error", err
	}

	if ctxt.HasParameter("m") {
		index := ctxt.QueryParamInt("m", -1)
		if index == -1 {
			return "error", EINVAL
		}
		delta := ctxt.QueryParamInt("n", 0)
		if delta == 0 {
			return "error", EINVAL
		}
		err = database.AmReorderSideboxes(ctxt.Ctx(), ctxt.CurrentUserId(), sboxes[index].Sequence, sboxes[index+delta].Sequence)
		if err != nil {
			return "error", err
		}
		tmp := sboxes[index]
		sboxes[index] = sboxes[index+delta]
		sboxes[index+delta] = tmp
	} else if ctxt.HasParameter("d") {
		index := ctxt.QueryParamInt("d", -1)
		if index == -1 {
			return "error", EINVAL
		}
		err = database.AmRemoveSidebox(ctxt.Ctx(), ctxt.CurrentUserId(), sboxes[index].Boxid)
		if err != nil {
			return "error", err
		}
		sboxes = append(sboxes[:index], sboxes[index+1:]...)
	}

	// Create the display list.
	avail := maps.Clone(sideboxRegistry)
	disp := make([]*DisplaySidebox, 0, len(sboxes))
	for _, sb := range sboxes {
		if dsb, ok := avail[sb.Boxid]; ok {
			disp = append(disp, dsb)
			delete(avail, sb.Boxid)
		} else {
			log.Errorf("TopPage: unknown sidebox ID %d", sb.Boxid)
		}
	}
	ctxt.VarMap().Set("sideboxes", disp)

	// Create the list of available sideboxes to add.
	addList := slices.Collect(maps.Values(avail))
	if len(addList) > 0 {
		slices.SortFunc(addList, func(a, b *DisplaySidebox) int {
			return strings.Compare(strings.ToLower(a.Title), strings.ToLower(b.Title))
		})
	}
	ctxt.VarMap().Set("addList", addList)

	ctxt.SetFrameTitle("Your Front Page Configuration")
	return "framed", "sideboxes.jet"
}

/* AddSidebox performs the "add sidebox" operation.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func AddSidebox(ctxt ui.AmContext) (string, any) {
	if ctxt.CurrentUser().IsAnon {
		return "error", ELOGIN
	}

	newId, err := ctxt.FormFieldInt("sbid")
	if err != nil {
		return "error", err
	}
	err = database.AmAppendSidebox(ctxt.Ctx(), ctxt.CurrentUserId(), int32(newId), nil)
	if err != nil {
		return "error", err
	}
	return "redirect", "/sideboxes"
}

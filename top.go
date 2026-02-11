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
	"reflect"

	"git.erbosoft.com/amy/amsterdam/config"
	"git.erbosoft.com/amy/amsterdam/database"
	"git.erbosoft.com/amy/amsterdam/ui"
	"github.com/CloudyKit/jet/v6"
	"github.com/labstack/echo/v4"
)

// RenderedSideboxItem is an item for display inside a rendered sidebox.
type RenderedSideboxItem struct {
	Text  string
	Text2 string
	Link  *string
	Flags map[string]bool
}

// LinkX dereferences the Link pointer safely.
func (item *RenderedSideboxItem) LinkX() string {
	if item.Link == nil {
		return ""
	}
	return *item.Link
}

// RenderedSidebox is the data for a single rendered sidebox.
type RenderedSidebox struct {
	TemplateName string
	Title        string
	Subtext      *string
	Items        []RenderedSideboxItem
	Flags        map[string]bool
}

/* buildCommunitiesSidebox creates the data for the "My/Featured Communities" sidebox.
 * Parameters:
 *     uid - UID of the user rendering the page.
 *     out - The RenderedSidebox to be built.
 *     in - The sidebox data from the database.
 * Returns:
 *     Standard Go error status.
 */
func buildCommunitiesSidebox(ctxt ui.AmContext, uid int32, out *RenderedSidebox, in *database.Sidebox) error {
	user, err := database.AmGetUser(ctxt.Ctx(), uid)
	if err == nil {
		var g *database.Globals
		g, err = database.AmGlobals(ctxt.Ctx())
		if err == nil {
			if user.IsAnon {
				out.Title = "Featured Communities"
			} else {
				out.Title = "Your Communities"
			}
			var l []*database.Community
			l, err = database.AmGetCommunitiesForUser(ctxt.Ctx(), uid)
			if err == nil {
				out.Items = make([]RenderedSideboxItem, len(l))
				for i, c := range l {
					out.Items[i].Text = c.Name
					lk := fmt.Sprintf("/comm/%s/profile", c.Alias)
					out.Items[i].Link = &lk
					out.Items[i].Flags = make(map[string]bool)
					var level uint16
					level, err = database.AmGetCommunityAccessLevel(ctxt.Ctx(), uid, c.Id)
					if err == nil && database.AmTestPermission("Community.ShowAdmin", level) {
						out.Items[i].Flags["admin"] = true
					}
				}
				out.Flags = make(map[string]bool)
				if user.IsAnon {
					out.Flags["canManage"] = false
					out.Flags["canCreate"] = false
				} else {
					out.Flags["canManage"] = true
					out.Flags["canCreate"] = user.BaseLevel >= uint16(g.CommunityCreateLevel)
				}
				out.TemplateName = "sb_ftrcomm.jet"
			}
		}
	}
	_ = in
	return err
}

/* buildFeaturedConferences creates the data for the "Featured Conferences" sidebox.
 * Parameters:
 *     ctxt - AmContext for the operation.
 *     uid - UID of the user rendering the page.
 *     out - The RenderedSidebox to be built.
 *     in - The sidebox data from the database.
 * Returns:
 *     Standard Go error status.
 */
func buildFeaturedConferences(ctxt ui.AmContext, uid int32, out *RenderedSidebox, in *database.Sidebox) error {
	user, err := database.AmGetUser(ctxt.Ctx(), uid)
	if err == nil {
		if user.IsAnon {
			out.Title = "Featured Conferences"
		} else {
			out.Title = "Your Conference Hotlist"
		}
		var hl []database.ConferenceHotlist
		hl, err := database.AmGetConferenceHotlist(ctxt.Ctx(), user)
		if err == nil {
			out.Items = make([]RenderedSideboxItem, len(hl))
			for i, h := range hl {
				comm, err := h.Community(ctxt.Ctx())
				if err != nil {
					break
				}
				conf, err := h.Conference(ctxt.Ctx())
				if err != nil {
					break
				}
				alias, err := conf.Aliases(ctxt.Ctx())
				if err != nil {
					break
				}
				out.Items[i].Text = conf.Name
				out.Items[i].Text2 = comm.Name
				lk := fmt.Sprintf("/comm/%s/conf/%s", comm.Alias, alias[0])
				out.Items[i].Link = &lk
				out.Items[i].Flags = make(map[string]bool)
				out.Items[i].Flags["new"] = false
				if !user.IsAnon {
					nnew, err := conf.UnreadMessages(ctxt.Ctx(), user)
					if err == nil {
						out.Items[i].Flags["new"] = (nnew > 0)
					}
				}
			}
			out.Flags = make(map[string]bool)
			out.Flags["canManage"] = !(user.IsAnon)
			out.TemplateName = "sb_ftrconf.jet"
		}
	}
	_ = in
	return err
}

/* buildUsersOnline creates the data for the "Users Online" sidebox.
 * Parameters:
 *     uid - UID of the user rendering the page.
 *     out - The RenderedSidebox to be built.
 *     in - The sidebox data from the database.
 * Returns:
 *     Standard Go error status.
 */
func buildUsersOnline(uid int32, out *RenderedSidebox, in *database.Sidebox) error {
	out.Title = "Users Online"
	out.TemplateName = "sb_online.jet"
	anons, users, maxUsers := ui.AmSessions()
	cap := len(users) + 1
	if anons > 0 {
		cap++
	}
	out.Items = make([]RenderedSideboxItem, cap)
	out.Items[0].Text = fmt.Sprintf("%d total (max %d)", len(users)+anons, maxUsers)
	out.Items[0].Flags = make(map[string]bool)
	out.Items[0].Flags["nobullet"] = true
	out.Items[0].Flags["bold"] = true
	b := 1
	if anons > 0 {
		out.Items[1].Text = fmt.Sprintf("Not logged in (%d)", anons)
		out.Items[1].Flags = make(map[string]bool)
		b++
	}
	for i, n := range users {
		out.Items[b+i].Text = n
		lk := fmt.Sprintf("/user/%s", n)
		out.Items[b+i].Link = &lk
		out.Items[b+i].Flags = make(map[string]bool)
		out.Items[b+i].Flags["bold"] = true
	}
	_ = uid
	_ = in
	return nil
}

/* buildRenderedSidebox creates a RenderedSidebox for the data in the database.
 * Parameters:
 *     uid - UID of the user rendering the page.
 *     out - The RenderedSidebox to be built.
 *     in - The sidebox data from the database.
 * Returns:
 *     Standard Go error status.
 */
func buildRenderedSidebox(ctxt ui.AmContext, uid int32, out *RenderedSidebox, in *database.Sidebox) error {
	switch in.Boxid {
	case 1:
		return buildCommunitiesSidebox(ctxt, uid, out, in)
	case 2:
		return buildFeaturedConferences(ctxt, uid, out, in)
	case 3:
		return buildUsersOnline(uid, out, in)
	default:
		return fmt.Errorf("unknown sidebox boxid: %d", in.Boxid)
	}
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
	topic := args.Get(0).Convert(reflect.TypeFor[*database.Topic]()).Interface().(*database.Topic)
	ctxt := args.Get(1).Convert(reflect.TypeFor[ui.AmContext]()).Interface().(ui.AmContext)
	link, _ := topic.Link(ctxt.Ctx(), "global")
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
	hdrs, err := database.AmGetPublishedPosts(ctxt.Ctx())
	if err != nil {
		return "error", err
	}

	ctxt.VarMap().Set("posts", hdrs)
	ctxt.VarMap().SetFunc("post_getText", templatePostText)
	ctxt.VarMap().SetFunc("post_getUserName", templateExtractUserName)
	ctxt.VarMap().SetFunc("post_topic", templateGetTopic)
	ctxt.VarMap().SetFunc("post_topicLink", templateTopicLink)

	// Retrieve the sideboxes and create the data to be presented.
	uid := ctxt.CurrentUserId()
	sboxes, err := database.AmGetSideboxes(ctxt.Ctx(), uid)
	if err != nil {
		return "error", err
	}

	rc := make([]RenderedSidebox, len(sboxes))
	for i, sb := range sboxes {
		err = buildRenderedSidebox(ctxt, uid, &(rc[i]), sb)
		if err != nil {
			return "error", err
		}
	}
	ctxt.VarMap().Set("sideboxes", rc)

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
	// Set the page title.
	ctxt.SetFrameTitle("About Amsterdam")
	return "framed", "about.jet"
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

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
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"git.erbosoft.com/amy/amsterdam/database"
	"git.erbosoft.com/amy/amsterdam/email"
	"git.erbosoft.com/amy/amsterdam/htmlcheck"
	"git.erbosoft.com/amy/amsterdam/ui"
	"github.com/CloudyKit/jet/v6"
	log "github.com/sirupsen/logrus"
)

/* Conferences displayes the list of conferences in a community.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func Conferences(ctxt ui.AmContext) (string, any, error) {
	comm := ctxt.CurrentCommunity()
	ctxt.VarMap().Set("commName", comm.Name)
	ctxt.VarMap().Set("commAlias", comm.Alias)
	ctxt.VarMap().Set("amsterdam_pageTitle", "Conference Listing: "+comm.Name)
	clist, err := database.AmGetCommunityConferences(ctxt.Ctx(), comm.Id,
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
	prefs, err := ctxt.CurrentUser().Prefs(ctxt.Ctx())
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
		}
	}
	if !trustSessionValues {
		ctxt.SetSession("topic.conf", conf.ConfId)
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

	topics, err := database.AmListTopics(ctxt.Ctx(), conf.ConfId, ctxt.CurrentUserId(), view, sort, false)
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}

	var hotlistTest bool = false
	if !ctxt.CurrentUser().IsAnon {
		hotlistTest, err = database.AmIsInHotlist(ctxt.Ctx(), ctxt.CurrentUser(), comm.Id, conf.ConfId)
		if err != nil {
			return ui.ErrorPage(ctxt, err)
		}
	}

	// create the "read new" URL
	urlStem := fmt.Sprintf("/comm/%s/conf/%s", comm.Alias, ctxt.GetScratch("currentAlias"))
	if !ctxt.CurrentUser().IsAnon {
		traverser := ui.NewTopicTraverser(topics)
		ctxt.SetSession("topic.traverser", traverser)
		firstTopic := traverser.FirstTopic()
		if firstTopic >= 1 {
			ctxt.VarMap().Set("urlReadNew", fmt.Sprintf("%s/r/%d", urlStem, firstTopic))
		}
	} else {
		ctxt.SetSession("topic.traverser", nil)
	}

	tz := prefs.Location()
	loc := prefs.Localizer()
	fdate := make([]string, len(topics))
	for i, t := range topics {
		fdate[i] = loc.Strftime("%x %X", t.LastUpdate.In(tz))
	}

	ctxt.VarMap().Set("isAnon", ctxt.CurrentUser().IsAnon)
	ctxt.VarMap().Set("canCreate", !ctxt.CurrentUser().IsAnon && conf.TestPermission("Conference.Create", myLevel))
	ctxt.VarMap().Set("showHotlist", !ctxt.CurrentUser().IsAnon && !hotlistTest)
	ctxt.VarMap().Set("conferenceName", conf.Name)
	ctxt.VarMap().Set("urlBack", fmt.Sprintf("/comm/%s/conf", comm.Alias))
	ctxt.VarMap().Set("urlStem", urlStem)
	ctxt.VarMap().Set("permalink", fmt.Sprintf("/go/%s!%s", comm.Alias, ctxt.GetScratch("currentAlias")))
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
	comm := ctxt.CurrentCommunity()
	conf := ctxt.GetScratch("currentConference").(*database.Conference)
	myLevel := ctxt.GetScratch("levelInConference").(uint16)
	if !conf.TestPermission("Conference.Create", myLevel) {
		ctxt.SetRC(http.StatusForbidden)
		return ui.ErrorPage(ctxt, errors.New("you are not permitted to create topics in this conference"))
	}
	ctxt.VarMap().Set("conferenceName", conf.Name)
	ctxt.VarMap().Set("urlStem", fmt.Sprintf("/comm/%s/conf/%s", comm.Alias, ctxt.GetScratch("currentAlias")))
	ctxt.VarMap().Set("topicName", "")
	pseud, err := conf.DefaultPseud(ctxt.Ctx(), ctxt.CurrentUser())
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	ctxt.VarMap().Set("pseud", pseud)
	ctxt.VarMap().Set("pb", "")
	ctxt.VarMap().Set("amsterdam_pageTitle", "Create New Topic")
	return "framed_template", "new_topic.jet", nil
}

/* NewTopic creates a new topic and posts the initial message.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func NewTopic(ctxt ui.AmContext) (string, any, error) {
	comm := ctxt.CurrentCommunity()
	conf := ctxt.GetScratch("currentConference").(*database.Conference)
	myLevel := ctxt.GetScratch("levelInConference").(uint16)
	if !conf.TestPermission("Conference.Create", myLevel) {
		ctxt.SetRC(http.StatusForbidden)
		return ui.ErrorPage(ctxt, errors.New("you are not permitted to create topics in this conference"))
	}

	urlStem := fmt.Sprintf("/comm/%s/conf/%s", comm.Alias, ctxt.GetScratch("currentAlias"))
	if ctxt.FormFieldIsSet("cancel") {
		return "redirect", urlStem, nil
	}
	if ctxt.FormFieldIsSet("preview") {
		// start by escaping the title
		checker, err := htmlcheck.AmNewHTMLChecker(ctxt.Ctx(), "escaper")
		if err != nil {
			return ui.ErrorPage(ctxt, err)
		}
		checker.Append(ctxt.FormField("title"))
		checker.Finish()
		v, _ := checker.Value()
		ctxt.VarMap().Set("topicName", v)

		// escape the pseud
		checker.Reset()
		checker.Append(ctxt.FormField("pseud"))
		checker.Finish()
		v, _ = checker.Value()
		ctxt.VarMap().Set("pseud", v)

		// escape the data
		postdata := ctxt.FormField("pb")
		checker.Reset()
		checker.Append(postdata)
		checker.Finish()
		v, _ = checker.Value()
		ctxt.VarMap().Set("pb", v)

		// run the preview
		checker, err = htmlcheck.AmNewHTMLChecker(ctxt.Ctx(), "preview")
		if err != nil {
			return ui.ErrorPage(ctxt, err)
		}
		checker.SetContext("PostLinkDecoderContext", database.AmCreatePostLinkContext(comm.Alias, ctxt.URLParam("cid"), conf.TopTopic+1))
		checker.Append(postdata)
		checker.Finish()
		v, _ = checker.Value()
		ctxt.VarMap().Set("previewPb", v)
		nErr, _ := checker.Counter("spelling")
		ctxt.VarMap().Set("nError", nErr)

		if ctxt.FormFieldIsSet("attach") {
			ctxt.VarMap().Set("attachFile", true)
		}
		ctxt.VarMap().Set("conferenceName", conf.Name)
		ctxt.VarMap().Set("urlStem", urlStem)
		ctxt.VarMap().Set("amsterdam_pageTitle", "Preview New Topic")
		return "framed_template", "new_topic.jet", nil
	}
	if ctxt.FormFieldIsSet("post1") {
		// start by checking the title and pseud
		checker, err := htmlcheck.AmNewHTMLChecker(ctxt.Ctx(), "post-pseud")
		if err != nil {
			return ui.ErrorPage(ctxt, err)
		}
		checker.Append(ctxt.FormField("title"))
		checker.Finish()
		topicName, _ := checker.Value()
		checker.Reset()
		checker.Append(ctxt.FormField("pseud"))
		checker.Finish()
		zeroPostPseud, _ := checker.Value()

		// now check the post data itself
		checker, err = htmlcheck.AmNewHTMLChecker(ctxt.Ctx(), "post-body")
		if err != nil {
			return ui.ErrorPage(ctxt, err)
		}
		checker.SetContext("PostLinkDecoderContext", database.AmCreatePostLinkContext(comm.Alias, ctxt.URLParam("cid"), conf.TopTopic+1))
		checker.Append(ctxt.FormField("pb"))
		checker.Finish()
		zeroPost, _ := checker.Value()
		lines, _ := checker.Lines()

		// Add the topic!
		topic, err := database.AmNewTopic(ctxt.Ctx(), conf, ctxt.CurrentUser(), topicName, zeroPostPseud, zeroPost, int32(lines), ctxt.RemoteIP())
		if err != nil {
			return ui.ErrorPage(ctxt, err)
		}

		if !ctxt.FormFieldIsSet("attach") {
			return "redirect", urlStem, nil // no attachment - just redisplay topic list
		}

		post, err := topic.GetPost(ctxt.Ctx(), 0) // get the initial post in the new topic
		if err != nil {
			return ui.ErrorPage(ctxt, err)
		}

		// go upload the attachment
		ctxt.VarMap().Set("target", urlStem)
		ctxt.VarMap().Set("post", post.PostId)
		ctxt.VarMap().Set("amsterdam_pageTitle", "Upload Attachment")
		return "framed_template", "attachment_upload.jet", nil
	}

	return ui.ErrorPage(ctxt, errors.New("invalid button clicked on form"))
}

/* breakRange breaks up a post range into two elements.
 * Parameters:
 *     topic - The topic within which the range is defined.
 *     into - The 2-element array into which the range will be filled.
 *     param - The range parameter to be broken up.
 *     sep - The separator character to use
 */
func breakRange(topic *database.Topic, into []int32, param string, sep string) error {
	rstr := strings.Split(param, sep)
	if len(rstr) == 0 {
		return fmt.Errorf("posts not found: %s in topic %d", param, topic.Number)
	}
	v, err := strconv.ParseInt(strings.TrimSpace(rstr[0]), 10, 32)
	if err != nil {
		return fmt.Errorf("posts not found: %s in topic %d", param, topic.Number)
	}
	into[0] = int32(v)
	if len(rstr) > 1 {
		if len(rstr[1]) == 0 {
			into[1] = -1
		} else {
			v, err = strconv.ParseInt(strings.TrimSpace(rstr[1]), 10, 32)
			if err != nil {
				return fmt.Errorf("posts not found: %s in topic %d", param, topic.Number)
			}
			into[1] = int32(v)
		}
	} else {
		into[1] = into[0]
	}
	if into[1] < 0 {
		into[1] = topic.TopMessage
	}
	if into[0] > into[1] {
		t := into[0]
		into[0] = into[1]
		into[1] = t
	}
	into[0] = max(into[0], 0)
	into[1] = min(into[1], topic.TopMessage)
	return nil
}

// templateExtractUserName extracts the user name from the post.
func templateExtractUserName(args jet.Arguments) reflect.Value {
	rc := "<<ERROR>>"
	post := args.Get(0).Convert(reflect.TypeFor[*database.PostHeader]()).Interface().(*database.PostHeader)
	ctxt := args.Get(1).Convert(reflect.TypeFor[ui.AmContext]()).Interface().(ui.AmContext)
	user, err := database.AmGetUser(ctxt.Ctx(), post.CreatorUid)
	if err == nil {
		rc = user.Username
	} else {
		log.Errorf("templateExtractUserName failed to get user #%d: %v", post.CreatorUid, err)
	}
	return reflect.ValueOf(rc)
}

// templatePostText gets the text of a post.
func templatePostText(args jet.Arguments) reflect.Value {
	post := args.Get(0).Convert(reflect.TypeFor[*database.PostHeader]()).Interface().(*database.PostHeader)
	ctxt := args.Get(1).Convert(reflect.TypeFor[ui.AmContext]()).Interface().(ui.AmContext)
	rc, err := post.Text(ctxt.Ctx())
	if err != nil {
		log.Errorf("templatePostText could not get post text from post #%d: %v", post.PostId, err)
		rc = ""
	}
	return reflect.ValueOf(rc)
}

// templateOverrideLine creates the "override line" for a post, that is, what gets displayed in place of the post text.
func templateOverrideLine(args jet.Arguments) reflect.Value {
	post := args.Get(0).Convert(reflect.TypeFor[*database.PostHeader]()).Interface().(*database.PostHeader)
	ctxt := args.Get(1).Convert(reflect.TypeFor[ui.AmContext]()).Interface().(ui.AmContext)
	rc := ""
	if post.IsScribbled() {
		scr_date := ""
		scr_user, err := database.AmGetUser(ctxt.Ctx(), *post.ScribbleUid)
		if err == nil {
			var p *database.UserPrefs
			p, err = ctxt.CurrentUser().Prefs(ctxt.Ctx())
			if err == nil {
				scr_date = p.Localizer().Strftime("%b %e, %Y %r", *post.ScribbleDate)
			}
		}
		if err == nil {
			rc = fmt.Sprintf("(Scribbled by %s on %s)", scr_user.Username, scr_date)
		} else {
			rc = fmt.Sprintf("<<<%v>>>", err)
		}
	} else if post.Hidden {
		rc = fmt.Sprintf("(Hidden Message: %d Lines)", *post.LineCount)
	}
	return reflect.ValueOf(rc)
}

// templateOverrideLink creates the "override link" for a post, which can make the override line a hyperlink.
func templateOverrideLink(args jet.Arguments) reflect.Value {
	post := args.Get(0).Convert(reflect.TypeFor[*database.PostHeader]()).Interface().(*database.PostHeader)
	root := args.Get(1).Convert(reflect.TypeFor[string]()).String()
	rc := ""
	if post.Hidden {
		rc = fmt.Sprintf("%s?r=%d&ac=1", root, post.Num)
	}
	return reflect.ValueOf(rc)
}

// templateAttachmentInfo gets the attachment info for a post.
func templateAttachmentInfo(args jet.Arguments) reflect.Value {
	post := args.Get(0).Convert(reflect.TypeFor[*database.PostHeader]()).Interface().(*database.PostHeader)
	ctxt := args.Get(1).Convert(reflect.TypeFor[ui.AmContext]()).Interface().(ui.AmContext)
	rc, _ := post.AttachmentInfo(ctxt.Ctx())
	if rc == nil {
		rc = &database.PostAttachInfo{}
	}
	return reflect.ValueOf(rc)
}

// templateBozo returns true if the post's creator user has been filtered by the current one.
func templateBozo(args jet.Arguments) reflect.Value {
	post := args.Get(0).Convert(reflect.TypeFor[*database.PostHeader]()).Interface().(*database.PostHeader)
	topic := args.Get(1).Convert(reflect.TypeFor[*database.Topic]()).Interface().(*database.Topic)
	ctxt := args.Get(2).Convert(reflect.TypeFor[ui.AmContext]()).Interface().(ui.AmContext)
	rc, _ := topic.IsBozo(ctxt.Ctx(), ctxt.CurrentUser(), post.CreatorUid)
	return reflect.ValueOf(rc)
}

/* ReadPosts displays posts in a topic.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func ReadPosts(ctxt ui.AmContext) (string, any, error) {
	// Extract the traverser first, so we can unclear it in background tasks.
	var traverser ui.TopicTraverser = nil
	if ctxt.IsSession("topic.traverser") {
		traverser = ctxt.GetSession("topic.traverser").(ui.TopicTraverser)
	}

	// If we need to reset a topic's last read count (as with "Next & Keep New"), spin the task off.
	if ctxt.HasParameter("rst") {
		rst := strings.Split(ctxt.Parameter("rst"), ",")
		if len(rst) >= 2 {
			user := ctxt.CurrentUser()
			ampool.Submit(func(ctx context.Context) {
				topicId, e1 := strconv.ParseInt(rst[0], 10, 32)
				lastRead, e2 := strconv.ParseInt(rst[1], 10, 32)
				if e1 == nil && e2 == nil {
					topic, _ := database.AmGetTopic(ctx, int32(topicId))
					if topic != nil {
						topic.SetLastRead(ctx, user, int32(lastRead))
						if traverser != nil && int32(lastRead) < topic.TopMessage {
							traverser.UnclearTopic(topic.Number)
						}
					}
				}
			})
		}
	}
	// Get user prefs.
	prefs, err := ctxt.CurrentUser().Prefs(ctxt.Ctx())
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	// Locate community, conference, and topic.
	comm := ctxt.CurrentCommunity()
	conf := ctxt.GetScratch("currentConference").(*database.Conference)
	myLevel := ctxt.GetScratch("levelInConference").(uint16)
	topic := ctxt.GetScratch("currentTopic").(*database.Topic)
	ctxt.VarMap().Set("post_topic", topic)

	// Determine the range of posts to display.  The "pin" is the post number after which we display the horizontal line separating old and new posts.
	lastRead, err := topic.GetLastRead(ctxt.Ctx(), ctxt.CurrentUser())
	if err != nil {
		ctxt.SetRC(http.StatusNotFound)
		return ui.ErrorPage(ctxt, fmt.Errorf("posts not found in topic %d - %v", topic.Number, err))
	}
	postRange := make([]int32, 2)
	var pin int32 = -1
	resetLastRead := false
	if ctxt.HasParameter("r") {
		if err := breakRange(topic, postRange, ctxt.Parameter("r"), ","); err != nil {
			ctxt.SetRC(http.StatusNotFound)
			return ui.ErrorPage(ctxt, err)
		}
	} else if ctxt.HasParameter("rgo") {
		if err := breakRange(topic, postRange, ctxt.Parameter("rgo"), "-"); err != nil {
			ctxt.SetRC(http.StatusNotFound)
			return ui.ErrorPage(ctxt, err)
		}
	} else {
		postRange[0] = lastRead + 1
		postRange[1] = topic.TopMessage
		count := postRange[1] - postRange[0] + 1
		if count == 0 {
			postRange[0] = max(postRange[1]-ctxt.Globals().PostsPerPage+1, 0)
		} else if count > ctxt.Globals().PostsPerPage {
			postRange[0] = postRange[1] - ctxt.Globals().PostsPerPage + 1
		} else if count < ctxt.Globals().PostsPerPage {
			pin = postRange[0] - 1
			postRange[0] = max(0, postRange[0]-ctxt.Globals().OldPostsAtTop)
			if pin < postRange[0] || pin >= postRange[1] {
				pin = -1
			}
		}
		resetLastRead = true
	}

	// Load the actual posts.
	posts, err := database.AmGetPostRange(ctxt.Ctx(), topic, postRange[0], postRange[1])
	if err != nil {
		return ui.ErrorPage(ctxt, fmt.Errorf("internal error getting posts <%d:%d-%d> - %v", topic.Number, postRange[0], postRange[1], err))
	}

	// Determine other required data.
	hidden, _ := topic.IsHidden(ctxt.Ctx(), ctxt.CurrentUser())
	ctxt.VarMap().Set("isTopicHidden", hidden)
	var flags strings.Builder
	if topic.Sticky {
		flags.WriteString("📌")
	}
	if hidden {
		flags.WriteString("🙈")
	}
	if topic.Archived {
		flags.WriteString("🗄️")
	} else if topic.Frozen {
		flags.WriteString("🧊")
	}
	summaryLine := fmt.Sprintf("%d Total; %d New; Last: %s", topic.TopMessage+1, topic.TopMessage-lastRead, prefs.Localizer().Strftime("%b %e, %Y %r", topic.LastUpdate))
	ctxt.VarMap().Set("summaryLine", flags.String()+summaryLine)
	ctxt.VarMap().Set("amsterdam_pageTitle", fmt.Sprintf("%s: %s%s", topic.Name, flags.String(), summaryLine))
	plc := database.AmCreatePostLinkContext("", ctxt.GetScratch("currentAlias").(string), topic.Number)
	ctxt.VarMap().Set("post_confRef", plc.AsString())
	plc.Community = comm.Alias
	ctxt.VarMap().Set("post_topicPermalink", fmt.Sprintf("/go/%s", plc.AsString()))
	plc.FirstPost = postRange[0]
	plc.LastPost = postRange[1]
	ctxt.VarMap().Set("postsPermalink", fmt.Sprintf("/go/%s", plc.AsString()))

	// Set the user's pseud.
	pseud, err := conf.DefaultPseud(ctxt.Ctx(), ctxt.CurrentUser())
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	ctxt.VarMap().Set("pseud", pseud)

	// Set permission and status flags.
	confHidePerm := conf.TestPermission("Conference.Hide", myLevel)
	ctxt.VarMap().Set("canFreeze", confHidePerm)
	ctxt.VarMap().Set("canArchive", confHidePerm)
	ctxt.VarMap().Set("canStick", confHidePerm)
	ctxt.VarMap().Set("isFrozen", topic.Frozen)
	ctxt.VarMap().Set("isArchived", topic.Archived)
	ctxt.VarMap().Set("isSticky", topic.Sticky)
	confNukePerm := conf.TestPermission("Conference.Nuke", myLevel)
	ctxt.VarMap().Set("canDelete", confNukePerm)
	confPostPerm := conf.TestPermission("Conference.Post", myLevel)
	ctxt.VarMap().Set("canPost", (!(topic.Frozen || topic.Archived) || confHidePerm) && confPostPerm)
	ctxt.VarMap().Set("showFrozenArchiveMessages", confPostPerm)

	// Set advanced controls.
	advancedControls := ctxt.HasParameter("ac") && (len(posts) == 1)
	if advancedControls {
		nbozo := ctxt.QueryParamInt("bozo", -1)
		if nbozo >= 0 {
			err = topic.SetBozo(ctxt.Ctx(), ctxt.CurrentUser(), posts[0].CreatorUid, nbozo != 0)
			if err != nil {
				return ui.ErrorPage(ctxt, err)
			}
		}
		isMyPost := (posts[0].CreatorUid == ctxt.CurrentUserId()) && !ctxt.CurrentUser().IsAnon
		isScribbled := posts[0].IsScribbled()
		canHide := !isScribbled && (isMyPost || confHidePerm)
		ctxt.VarMap().Set("canHide", canHide)
		ctxt.VarMap().Set("isPostHidden", posts[0].Hidden)
		canScribble := !isScribbled && (isMyPost || confNukePerm)
		ctxt.VarMap().Set("canScribble", canScribble)
		ctxt.VarMap().Set("canNuke", confNukePerm)
		canPublish := !isScribbled && database.AmTestPermission("Global.PublishFP", myLevel)
		if canPublish {
			published, _ := posts[0].IsPublished(ctxt.Ctx())
			if published {
				canPublish = false
			}
		}
		ctxt.VarMap().Set("canPublish", canPublish)
	}
	ctxt.VarMap().Set("advancedControls", advancedControls)

	// Adjust the traverser and get the "next" link.
	urlStem := fmt.Sprintf("/comm/%s/conf/%s", comm.Alias, ctxt.GetScratch("currentAlias").(string))
	if traverser != nil {
		traverser.ClearTopic(topic.Number)
		nextTopic := traverser.NextTopic(topic.Number)
		if nextTopic >= 0 {
			nextTopicURL := fmt.Sprintf("%s/r/%d", urlStem, nextTopic)
			ctxt.VarMap().Set("urlNextTopic", nextTopicURL)
			ctxt.VarMap().Set("urlNextKeepNew", fmt.Sprintf("%s?rst=%d,%d", nextTopicURL, topic.TopicId, lastRead))
		}
	}

	// Render the output.
	ctxt.VarMap().Set("isAnon", ctxt.CurrentUser().IsAnon)
	ctxt.VarMap().Set("topicName", topic.Name)
	ctxt.VarMap().Set("lastRead", lastRead)
	ctxt.VarMap().Set("pageSize", ctxt.Globals().PostsPerPage)
	ctxt.VarMap().SetFunc("post_getOverrideLine", templateOverrideLine)
	ctxt.VarMap().SetFunc("post_getOverrideLink", templateOverrideLink)
	ctxt.VarMap().SetFunc("post_getText", templatePostText)
	ctxt.VarMap().SetFunc("post_getUserName", templateExtractUserName)
	ctxt.VarMap().SetFunc("post_getAttachmentInfo", templateAttachmentInfo)
	ctxt.VarMap().SetFunc("post_isBozo", templateBozo)
	ctxt.VarMap().Set("post_stem", fmt.Sprintf("%s/r/%d", urlStem, topic.Number))
	ctxt.VarMap().Set("post_max", topic.TopMessage)
	ctxt.VarMap().Set("posts", posts)
	ctxt.VarMap().Set("pin", pin)
	ctxt.VarMap().Set("rangeEnd", postRange[1])
	ctxt.VarMap().Set("rangeStart", postRange[0])
	ctxt.VarMap().Set("topicListLink", urlStem)
	ctxt.VarMap().Set("topicNum", topic.Number)
	if resetLastRead {
		user := ctxt.CurrentUser()
		ampool.Submit(func(ctx context.Context) {
			topic.SetLastRead(ctx, user, topic.TopMessage)
		})
	}
	return "framed_template", "posts.jet", nil
}

/* PostInTopic adds a new post to a topic.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func PostInTopic(ctxt ui.AmContext) (string, any, error) {
	// Locate community, conference, and topic.
	comm := ctxt.CurrentCommunity()
	conf := ctxt.GetScratch("currentConference").(*database.Conference)
	level := ctxt.GetScratch("levelInConference").(uint16)
	topic := ctxt.GetScratch("currentTopic").(*database.Topic)
	ctxt.VarMap().Set("post_topic", topic)

	urlStem := fmt.Sprintf("/comm/%s/conf/%s/r/%d", comm.Alias, ctxt.GetScratch("currentAlias"), topic.Number)
	if ctxt.FormFieldIsSet("cancel") {
		return "redirect", urlStem, nil
	}

	if !conf.TestPermission("Conference.Post", level) {
		ctxt.SetRC(http.StatusForbidden)
		return ui.ErrorPage(ctxt, errors.New("you do not have permission to post in this conference"))
	}

	if topic.Frozen && !conf.TestPermission("Conference.Hide", level) {
		ctxt.SetRC(http.StatusForbidden)
		return ui.ErrorPage(ctxt, errors.New("this topic is frozen, and you do not have permission to post to it"))
	}

	if topic.Archived && !conf.TestPermission("Conference.Hide", level) {
		ctxt.SetRC(http.StatusForbidden)
		return ui.ErrorPage(ctxt, errors.New("this topic is archived, and you do not have permission to post to it"))
	}

	// Set the escaped version of the text into the varmap, because it'll be needed if we do anything other than redirect.
	checker, err := htmlcheck.AmNewHTMLChecker(ctxt.Ctx(), "escaper")
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	checker.Append(ctxt.FormField("pseud"))
	checker.Finish()
	v, _ := checker.Value()
	ctxt.VarMap().Set("pseud", v)
	postdata := ctxt.FormField("pb")
	checker.Reset()
	checker.Append(postdata)
	checker.Finish()
	v, _ = checker.Value()
	ctxt.VarMap().Set("pb", v)

	// also set the "attach" flag and "next topic" link (where applicable) into the post data
	if ctxt.FormFieldIsSet("attach") {
		ctxt.VarMap().Set("attachFile", true)
	}
	urlNextTopic := ctxt.FormField("nextlink")
	if len(urlNextTopic) > 0 {
		ctxt.VarMap().Set("urlNextTopic", urlNextTopic)
	}

	if ctxt.FormFieldIsSet("preview") {
		// Preview the post.
		checker, err = htmlcheck.AmNewHTMLChecker(ctxt.Ctx(), "preview")
		if err != nil {
			return ui.ErrorPage(ctxt, err)
		}
		checker.SetContext("PostLinkDecoderContext", database.AmCreatePostLinkContext(comm.Alias, ctxt.URLParam("cid"), topic.Number))
		checker.Append(postdata)
		checker.Finish()
		v, _ = checker.Value()
		ctxt.VarMap().Set("previewPb", v)
		nErr, _ := checker.Counter("spelling")
		ctxt.VarMap().Set("nError", nErr)

		ctxt.VarMap().Set("maxPost", ctxt.FormField("xp"))
		ctxt.VarMap().Set("urlStem", urlStem)
		ctxt.VarMap().Set("amsterdam_pageTitle", "Previewing Message")
		return "framed_template", "preview_post.jet", nil
	}
	// Figure out which URL to return to once this post is made.
	var returnURL string
	if ctxt.FormFieldIsSet("post") {
		returnURL = urlStem
	} else if ctxt.FormFieldIsSet("postnext") && len(urlNextTopic) > 0 {
		returnURL = urlNextTopic
	} else if ctxt.FormFieldIsSet("posttopics") {
		returnURL = fmt.Sprintf("/comm/%s/conf/%s", comm.Alias, ctxt.GetScratch("currentAlias"))
	} else {
		return ui.ErrorPage(ctxt, errors.New("unknown post button"))
	}

	// Check for slippage.
	maxPost, err := ctxt.FormFieldInt("xp")
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	if int32(maxPost) < topic.TopMessage {
		// Slippage detected! Display the slipped posts and another post box.
		// Start by getting the slipped posts.
		posts, err := database.AmGetPostRange(ctxt.Ctx(), topic, int32(maxPost), topic.TopMessage)
		if err != nil {
			return ui.ErrorPage(ctxt, err)
		}

		plc := database.AmCreatePostLinkContext("", ctxt.GetScratch("currentAlias").(string), topic.Number)
		ctxt.VarMap().Set("post_confRef", plc.AsString())
		plc.Community = comm.Alias
		ctxt.VarMap().Set("post_topicPermalink", fmt.Sprintf("/go/%s", plc.AsString()))

		ctxt.VarMap().SetFunc("post_getOverrideLine", templateOverrideLine)
		ctxt.VarMap().SetFunc("post_getOverrideLink", templateOverrideLink)
		ctxt.VarMap().SetFunc("post_getText", templatePostText)
		ctxt.VarMap().SetFunc("post_getUserName", templateExtractUserName)
		ctxt.VarMap().SetFunc("post_getAttachmentInfo", templateAttachmentInfo)
		ctxt.VarMap().SetFunc("post_isBozo", templateBozo)
		ctxt.VarMap().Set("post_stem", fmt.Sprintf("/comm/%s/conf/%s/r/%d", comm.Alias, ctxt.GetScratch("currentAlias"), topic.Number))
		ctxt.VarMap().Set("post_max", topic.TopMessage)
		ctxt.VarMap().Set("posts", posts)
		ctxt.VarMap().Set("topicName", topic.Name)
		ctxt.VarMap().Set("amsterdam_pageTitle", "Slippage or Double-Click Detected")

		return "framed_template", "slippage.jet", nil
	}
	// if we get here, we are posting - start by checking the title and pseud
	checker, err = htmlcheck.AmNewHTMLChecker(ctxt.Ctx(), "post-pseud")
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	checker.Append(ctxt.FormField("pseud"))
	checker.Finish()
	postPseud, _ := checker.Value()

	// now check the post data itself
	checker, err = htmlcheck.AmNewHTMLChecker(ctxt.Ctx(), "post-body")
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	checker.SetContext("PostLinkDecoderContext", database.AmCreatePostLinkContext(comm.Alias, ctxt.URLParam("cid"), topic.Number))
	checker.Append(ctxt.FormField("pb"))
	checker.Finish()
	postText, _ := checker.Value()
	lines, _ := checker.Lines()

	// Add the post!
	hdr, err := database.AmNewPost(ctxt.Ctx(), conf, topic, ctxt.CurrentUser(), postPseud, postText, int32(lines), ctxt.RemoteIP())
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}

	// Check who's subscribed to this topic.
	subs, err := topic.GetSubscribers(ctxt.Ctx())
	if err != nil {
		log.Errorf("unable to deliver message to subscribers: %v", err)
	} else if len(subs) > 0 {
		// kick off a task to compose E-mails and deliver them to everyone
		alias := ctxt.GetScratch("currentAlias").(string)
		poster := ctxt.CurrentUser()
		ipaddr := ctxt.RemoteIP()
		ampool.Submit(func(ctx context.Context) {
			email.AmDeliverSubscription(ctx, comm, conf, alias, topic, poster, hdr, postText, subs, ipaddr)
		})
	}

	if !ctxt.FormFieldIsSet("attach") {
		return "redirect", returnURL, nil // no attachment - just bounce directly to the destination
	}

	// go upload the attachment
	ctxt.VarMap().Set("target", returnURL)
	ctxt.VarMap().Set("post", hdr.PostId)
	ctxt.VarMap().Set("amsterdam_pageTitle", "Upload Attachment")
	return "framed_template", "attachment_upload.jet", nil
}

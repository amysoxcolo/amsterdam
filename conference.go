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
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"git.erbosoft.com/amy/amsterdam/database"
	"git.erbosoft.com/amy/amsterdam/htmlcheck"
	"git.erbosoft.com/amy/amsterdam/ui"
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
	comm := ctxt.CurrentCommunity()
	conf := ctxt.GetScratch("currentConference").(*database.Conference)
	myLevel := ctxt.GetScratch("levelInConference").(uint16)
	if !conf.TestPermission("Conference.Create", myLevel) {
		ctxt.SetRC(http.StatusForbidden)
		return ui.ErrorPage(ctxt, errors.New("you are not permitted to create topics in this conference"))
	}
	ctxt.VarMap().Set("conferenceName", conf.Name)
	ctxt.VarMap().Set("urlStem", fmt.Sprintf("/comm/%s/conf/%s", comm.Alias, ctxt.URLParam("confid")))
	ctxt.VarMap().Set("topicName", "")
	cs, err := conf.Settings(ctxt.CurrentUser())
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	if cs == nil || cs.DefaultPseud == nil {
		ci, err := ctxt.CurrentUser().ContactInfo()
		if err != nil {
			return ui.ErrorPage(ctxt, err)
		}
		ctxt.VarMap().Set("pseud", ci.FullName(false))
	} else {
		ctxt.VarMap().Set("pseud", *cs.DefaultPseud)
	}
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

	urlStem := fmt.Sprintf("/comm/%s/conf/%s", comm.Alias, ctxt.URLParam("confid"))
	if ctxt.FormFieldIsSet("cancel") {
		return "redirect", urlStem, nil
	}
	if ctxt.FormFieldIsSet("preview") {
		// start by escaping the title
		checker, err := htmlcheck.AmNewHTMLChecker("escaper")
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
		checker, err = htmlcheck.AmNewHTMLChecker("preview")
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
		checker, err := htmlcheck.AmNewHTMLChecker("post-pseud")
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
		checker, err = htmlcheck.AmNewHTMLChecker("post-body")
		if err != nil {
			return ui.ErrorPage(ctxt, err)
		}
		checker.SetContext("PostLinkDecoderContext", database.AmCreatePostLinkContext(comm.Alias, ctxt.URLParam("cid"), conf.TopTopic+1))
		checker.Append(ctxt.FormField("pb"))
		checker.Finish()
		zeroPost, _ := checker.Value()
		lines, _ := checker.Lines()

		// Add the topic!
		topic, err := database.AmNewTopic(conf, ctxt.CurrentUser(), topicName, zeroPostPseud, zeroPost, int32(lines), ctxt.RemoteIP())
		if err != nil {
			return ui.ErrorPage(ctxt, err)
		}

		if !ctxt.FormFieldIsSet("attach") {
			return "redirect", urlStem, nil // no attachment - just redisplay topic list
		}

		post, err := topic.GetPost(0) // get the initial post in the new topic
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

// slurpFile reads the contrents of a multipart.File into memory.
func slurpFile(file *multipart.FileHeader) ([]byte, error) {
	f, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

/* AttachmentUpload adds an attachment to a post.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func AttachmentUpload(ctxt ui.AmContext) (string, any, error) {
	target := ctxt.FormField("tgt")
	postidStr := ctxt.FormField("post")
	postId, err := strconv.ParseInt(postidStr, 10, 64)
	if err != nil {
		return ui.ErrorPage(ctxt, fmt.Errorf("internal error converting postID: %v", err))
	}
	if ctxt.FormFieldIsSet("upload") {
		file, err := ctxt.FormFile("thefile")
		if err == nil {
			if file.Size <= (1024 * 1024) { // 1 Mb
				var post *database.PostHeader
				post, err = database.AmGetPost(postId)
				if err == nil {
					var data []byte
					data, err = slurpFile(file)
					if err == nil {
						err = post.SetAttachment(file.Filename, file.Header.Get("Content-Type"), int32(file.Size), data)
						if err == nil {
							return "redirect", target, nil
						}
					}
				}
			} else {
				err = errors.New("the file is too large to be attached")
			}
		}

		ctxt.VarMap().Set("target", target)
		ctxt.VarMap().Set("post", postId)
		ctxt.VarMap().Set("amsterdam_pageTitle", "Upload Attachment")
		ctxt.VarMap().Set("errorMessage", err.Error())
		return "framed_template", "attachment_upload.jet", nil
	}
	return ui.ErrorPage(ctxt, errors.New("invalid button clicked on form"))
}

func breakRange(topic *database.Topic, into []int32, param string, sep string) error {
	rstr := strings.Split(param, sep)
	if len(rstr) == 0 {
		return fmt.Errorf("posts not found: %s in topic %d", param, topic.Number)
	}
	v, err := strconv.ParseInt(rstr[0], 10, 32)
	if err != nil {
		return fmt.Errorf("posts not found: %s in topic %d", param, topic.Number)
	}
	into[0] = int32(v)
	if len(rstr) > 1 {
		v, err = strconv.ParseInt(rstr[1], 10, 32)
		if err != nil {
			return fmt.Errorf("posts not found: %s in topic %d", param, topic.Number)
		}
		into[1] = int32(v)
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

func ReadPosts(ctxt ui.AmContext) (string, any, error) {
	// If we need to reset a topic's last read count (as with "Next & Keep New"), spin the task off.
	if ctxt.HasParameter("rst") {
		rst := strings.Split(ctxt.Parameter("rst"), ",")
		if len(rst) >= 2 {
			user := ctxt.CurrentUser()
			ampool.Submit(func(context.Context) {
				topicId, e1 := strconv.ParseInt(rst[0], 10, 32)
				lastRead, e2 := strconv.ParseInt(rst[1], 10, 32)
				if e1 == nil && e2 == nil {
					topic, _ := database.AmGetTopic(int32(topicId))
					if topic != nil {
						topic.SetLastRead(user, int32(lastRead))
					}
				}
			})
		}
	}
	// Get user prefs.
	prefs, err := ctxt.CurrentUser().Prefs()
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	// Locate community, conference, and topic.
	comm := ctxt.CurrentCommunity()
	conf := ctxt.GetScratch("currentConference").(*database.Conference)
	var topic *database.Topic = nil
	if rawTopic, err := strconv.ParseInt(ctxt.URLParam("topic"), 10, 16); err == nil {
		topic, err = database.AmGetTopicByNumber(conf, int16(rawTopic))
	}
	if topic == nil {
		ctxt.SetRC(http.StatusNotFound)
		return ui.ErrorPage(ctxt, fmt.Errorf("topic not found: %s", ctxt.URLParam("topic")))
	}

	// Determine the range of posts to display.  The "pin" is the post number after which we display the horizontal line separating old and new posts.
	lastRead, err := topic.GetLastRead(ctxt.CurrentUser())
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
		if count > ctxt.Globals().PostsPerPage {
			postRange[0] = postRange[1] - ctxt.Globals().PostsPerPage + 1
		} else if count < ctxt.Globals().PostsPerPage {
			pin = postRange[0] - 1
			postRange[0] = max(0, postRange[0]-ctxt.Globals().OldPostsAtTop)
			if pin < postRange[0] {
				pin = -1
			}
		}
		resetLastRead = true
	}

	// Load the actual posts.
	posts, err := database.AmGetPostRange(topic, postRange[0], postRange[1])
	if err != nil {
		return ui.ErrorPage(ctxt, fmt.Errorf("internal error getting posts <%d:%d-%d> - %v", topic.Number, postRange[0], postRange[1], err))
	}

	// Determine other required data.
	loc := prefs.Localizer()
	summaryLine := fmt.Sprintf("%d Total; %d New; Last: %s", topic.TopMessage+1, topic.TopMessage-lastRead, loc.Strftime("%b %e, %Y %r", topic.LastUpdate))
	plc := database.AmCreatePostLinkContext(comm.Alias, ctxt.GetScratch("currentAlias").(string), topic.Number)
	topicPostRef := plc.AsString()
	plc.FirstPost = postRange[0]
	plc.LastPost = postRange[1]
	postsPostRef := plc.AsString()

	// Render the output.
	ctxt.VarMap().Set("stem", fmt.Sprintf("/comm/%s/conf/%s", comm.Alias, ctxt.GetScratch("currentAlias").(string)))
	ctxt.VarMap().Set("topicName", topic.Name)
	ctxt.VarMap().Set("summaryLine", summaryLine)
	ctxt.VarMap().Set("lastRead", lastRead)
	ctxt.VarMap().Set("pageSize", ctxt.Globals().PostsPerPage)
	ctxt.VarMap().Set("post_max", topic.TopMessage)
	ctxt.VarMap().Set("posts", posts)
	ctxt.VarMap().Set("postsPermalink", fmt.Sprintf("/go/%s", postsPostRef))
	ctxt.VarMap().Set("pin", pin)
	ctxt.VarMap().Set("rangeEnd", postRange[1])
	ctxt.VarMap().Set("rangeStart", postRange[0])
	ctxt.VarMap().Set("topicNum", topic.Number)
	ctxt.VarMap().Set("topicPermalink", fmt.Sprintf("/go/%s", topicPostRef))
	ctxt.VarMap().Set("amsterdam_pageTitle", fmt.Sprintf("%s: %s", topic.Name, summaryLine))
	if resetLastRead {
		user := ctxt.CurrentUser()
		ampool.Submit(func(context.Context) {
			topic.SetLastRead(user, topic.TopMessage)
		})
	}
	return "framed_template", "posts.jet", nil
}

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
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"git.erbosoft.com/amy/amsterdam/database"
	"git.erbosoft.com/amy/amsterdam/ui"
)

var ENOPERM error = errors.New("you are not permitted to perform this operation")

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
			var post *database.PostHeader
			post, err = database.AmGetPost(ctxt.Ctx(), postId)
			if err == nil {
				var data []byte
				data, err = slurpFile(file)
				if err == nil {
					err = post.SetAttachment(ctxt.Ctx(), ctxt.CurrentUser(), file.Filename, file.Header.Get("Content-Type"), int32(file.Size), data, ctxt.RemoteIP())
					if err == nil {
						return "redirect", target, nil
					}
				}
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

/* AttachmentSend sends the data of an attachment to the browser.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func AttachmentSend(ctxt ui.AmContext) (string, any, error) {
	postIdStr := ctxt.URLParam("post")
	postId, err := strconv.ParseInt(postIdStr, 10, 64)
	if err != nil {
		return ui.ErrorPage(ctxt, fmt.Errorf("internal error converting postID: %v", err))
	}

	hdr, err := database.AmGetPost(ctxt.Ctx(), postId)
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}

	// Retrieve attachment info and data.
	info, err := hdr.AttachmentInfo(ctxt.Ctx())
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	} else if info == nil {
		return ui.ErrorPage(ctxt, errors.New("attachment not found"))
	}
	data, err := hdr.AttachmentData(ctxt.Ctx())
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}

	// Record a "hit" on this attachment in the background.
	ampool.Submit(func(ctx context.Context) {
		hdr.HitAttachment(ctx)
	})

	// Send the attachment data.
	ctxt.SetOutputType(info.MIMEType)
	if !strings.HasPrefix(info.MIMEType, "image/") {
		ctxt.SetHeader("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", info.Filename))
	}
	return "bytes", data, nil
}

/* ConfManage displays the "manage conference" page.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func ConfManage(ctxt ui.AmContext) (string, any, error) {
	comm := ctxt.CurrentCommunity()
	conf := ctxt.GetScratch("currentConference").(*database.Conference)
	myLevel := ctxt.GetScratch("levelInConference").(uint16)
	urlStem := fmt.Sprintf("/comm/%s/conf/%s", comm.Alias, ctxt.GetScratch("currentAlias"))
	ctxt.VarMap().Set("confName", conf.Name)
	ctxt.VarMap().Set("urlStem", urlStem)

	pseud, err := conf.DefaultPseud(ctxt.Ctx(), ctxt.CurrentUser())
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	ctxt.VarMap().Set("pseud", pseud)

	if ctxt.CurrentUser().IsAnon {
		ctxt.VarMap().Set("canInvite", false)
	} else {
		member, _, _, err := comm.Membership(ctxt.Ctx(), ctxt.CurrentUser())
		if err != nil {
			return ui.ErrorPage(ctxt, err)
		}
		ctxt.VarMap().Set("canInvite", member)
	}

	if conf.TestPermission("Conference.Change", myLevel) || conf.TestPermission("Conference.Delete", myLevel) {
		menu := ui.AmMenu("confhost").FilterConference(comm, ctxt.GetScratch("currentAlias").(string))
		ctxt.VarMap().Set("menu", menu)
	}

	ctxt.VarMap().Set("amsterdam_pageTitle", "Manage Conference: "+conf.Name)
	return "framed_template", "manage_conf.jet", nil
}

/* AddToHotlist adds the current community and conference to the user's hotlist..
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func AddToHotlist(ctxt ui.AmContext) (string, any, error) {
	comm := ctxt.CurrentCommunity()
	conf := ctxt.GetScratch("currentConference").(*database.Conference)
	err := database.AmAppendToHotlist(ctxt.Ctx(), ctxt.CurrentUser(), comm.Id, conf.ConfId)
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	return "redirect", fmt.Sprintf("/comm/%s/conf/%s", comm.Alias, ctxt.GetScratch("currentAlias")), nil
}

/* HideTopic hides or shows the current topic for the current user.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func HideTopic(ctxt ui.AmContext) (string, any, error) {
	topic := ctxt.GetScratch("currentTopic").(*database.Topic)
	hidden, err := topic.IsHidden(ctxt.Ctx(), ctxt.CurrentUser())
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	err = topic.SetHidden(ctxt.Ctx(), ctxt.CurrentUser(), !hidden)
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	return "redirect", fmt.Sprintf("/comm/%s/conf/%s/r/%d", ctxt.CurrentCommunity().Alias, ctxt.GetScratch("currentAlias"), topic.Number), nil
}

/* HideMessage hides or shows a topic message.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func HideMessage(ctxt ui.AmContext) (string, any, error) {
	if ctxt.CurrentUser().IsAnon {
		ctxt.SetRC(http.StatusForbidden)
		return ui.ErrorPage(ctxt, ENOPERM)
	}
	conf := ctxt.GetScratch("currentConference").(*database.Conference)
	myLevel := ctxt.GetScratch("levelInConference").(uint16)
	topic := ctxt.GetScratch("currentTopic").(*database.Topic)
	msgNum, err := strconv.Atoi(ctxt.URLParam("msg"))
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	hdrs, err := database.AmGetPostRange(ctxt.Ctx(), topic, int32(msgNum), int32(msgNum))
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	} else if len(hdrs) != 1 {
		return ui.ErrorPage(ctxt, errors.New("internal error getting post reference"))
	}
	if (hdrs[0].CreatorUid != ctxt.CurrentUserId()) && !conf.TestPermission("Conference.Hide", myLevel) {
		ctxt.SetRC(http.StatusForbidden)
		return ui.ErrorPage(ctxt, ENOPERM)
	}
	err = hdrs[0].SetHidden(ctxt.Ctx(), ctxt.CurrentUser(), !(hdrs[0].Hidden), ctxt.RemoteIP())
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	return "redirect", fmt.Sprintf("/comm/%s/conf/%s/r/%d?r=%d&ac=1", ctxt.CurrentCommunity().Alias, ctxt.GetScratch("currentAlias"), topic.Number, hdrs[0].Num), nil
}

/* ScribbleMessage scribbles a topic message.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func ScribbleMessage(ctxt ui.AmContext) (string, any, error) {
	if ctxt.CurrentUser().IsAnon {
		ctxt.SetRC(http.StatusForbidden)
		return ui.ErrorPage(ctxt, ENOPERM)
	}
	conf := ctxt.GetScratch("currentConference").(*database.Conference)
	myLevel := ctxt.GetScratch("levelInConference").(uint16)
	topic := ctxt.GetScratch("currentTopic").(*database.Topic)
	msgNum, err := strconv.Atoi(ctxt.URLParam("msg"))
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	hdrs, err := database.AmGetPostRange(ctxt.Ctx(), topic, int32(msgNum), int32(msgNum))
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	} else if len(hdrs) != 1 {
		return ui.ErrorPage(ctxt, errors.New("internal error getting post reference"))
	}
	if (hdrs[0].CreatorUid != ctxt.CurrentUserId()) && !conf.TestPermission("Conference.Nuke", myLevel) {
		ctxt.SetRC(http.StatusForbidden)
		return ui.ErrorPage(ctxt, ENOPERM)
	}
	err = hdrs[0].Scribble(ctxt.Ctx(), ctxt.CurrentUser(), ctxt.RemoteIP())
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	return "redirect", fmt.Sprintf("/comm/%s/conf/%s/r/%d?r=%d&ac=1", ctxt.CurrentCommunity().Alias, ctxt.GetScratch("currentAlias"), topic.Number, hdrs[0].Num), nil
}

/* NukeMessage nukes (deletes entirely) a topic message.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func NukeMessage(ctxt ui.AmContext) (string, any, error) {
	if ctxt.CurrentUser().IsAnon {
		ctxt.SetRC(http.StatusForbidden)
		return ui.ErrorPage(ctxt, ENOPERM)
	}
	conf := ctxt.GetScratch("currentConference").(*database.Conference)
	myLevel := ctxt.GetScratch("levelInConference").(uint16)
	topic := ctxt.GetScratch("currentTopic").(*database.Topic)
	msgNum, err := strconv.Atoi(ctxt.URLParam("msg"))
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	hdrs, err := database.AmGetPostRange(ctxt.Ctx(), topic, int32(msgNum), int32(msgNum))
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	} else if len(hdrs) != 1 {
		return ui.ErrorPage(ctxt, errors.New("internal error getting post reference"))
	}
	if !conf.TestPermission("Conference.Nuke", myLevel) {
		ctxt.SetRC(http.StatusForbidden)
		return ui.ErrorPage(ctxt, ENOPERM)
	}

	// Load the message box, and, if we have a valid "yes," then perform the nuke!
	mbox, err := ui.AmLoadMessageBox("nuke")
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	if mbox.Validate(ctxt, "yes") {
		// do the nuking!
		err := hdrs[0].Nuke(ctxt.Ctx(), ctxt.CurrentUser(), ctxt.RemoteIP())
		if err != nil {
			return ui.ErrorPage(ctxt, err)
		}
		return "redirect", fmt.Sprintf("/comm/%s/conf/%s/r/%d", ctxt.CurrentCommunity().Alias, ctxt.GetScratch("currentAlias"), topic.Number), nil
	}

	// Set up to display the message box.
	link, err := hdrs[0].Link(ctxt.Ctx(), "community")
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	creator, err := hdrs[0].Creator(ctxt.Ctx())
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	mbox.SetMessage(fmt.Sprintf(`You are about to nuke message <span class="font-mono font-bold text-red-600">&lt;%s&gt;</span>, 
                        		originally composed by <span class="font-bold text-red-600">&lt;%s&gt;</span>!`, link, creator.Username))
	mbox.SetLink("no", fmt.Sprintf("/comm/%s/conf/%s/r/%d?r=%d&ac=1", ctxt.CurrentCommunity().Alias, ctxt.GetScratch("currentAlias"), topic.Number, hdrs[0].Num))
	mbox.SetLink("yes", fmt.Sprintf("/comm/%s/conf/%s/op/%d/nuke/%d", ctxt.CurrentCommunity().Alias, ctxt.GetScratch("currentAlias"), topic.Number, hdrs[0].Num))
	return mbox.Render(ctxt)
}

/* TopicManage displays the "manage topic" page.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func TopicManage(ctxt ui.AmContext) (string, any, error) {
	comm := ctxt.CurrentCommunity()
	topic := ctxt.GetScratch("currentTopic").(*database.Topic)
	ctxt.VarMap().Set("backlink", fmt.Sprintf("/comm/%s/conf/%s/r/%d", comm.Alias, ctxt.GetScratch("currentAlias"), topic.Number))
	opsLink := fmt.Sprintf("/comm/%s/conf/%s/op/%d", comm.Alias, ctxt.GetScratch("currentAlias"), topic.Number)
	ctxt.VarMap().Set("opsLink", opsLink)
	ctxt.VarMap().Set("topicName", topic.Name)

	// Get the invitation flag.
	member, _, _, err := comm.Membership(ctxt.Ctx(), ctxt.CurrentUser())
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	ctxt.VarMap().Set("canInvite", member)

	ctxt.VarMap().Set("subscribed", false)        // TODO
	ctxt.VarMap().Set("bozos", make([]string, 0)) // TODO

	ctxt.VarMap().Set("amsterdam_pageTitle", "Manage Topic: "+topic.Name)
	return "framed_template", "manage_topic.jet", nil
}

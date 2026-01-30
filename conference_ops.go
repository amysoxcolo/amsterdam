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
	"git.erbosoft.com/amy/amsterdam/email"
	"git.erbosoft.com/amy/amsterdam/ui"
	log "github.com/sirupsen/logrus"
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

/* SetPseud sets the user's default pseud for the conference.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func SetPseud(ctxt ui.AmContext) (string, any, error) {
	comm := ctxt.CurrentCommunity()
	conf := ctxt.GetScratch("currentConference").(*database.Conference)
	pseud := ctxt.FormField("pseud")
	err := conf.SetDefaultPseud(ctxt.Ctx(), ctxt.CurrentUser(), pseud)
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	return "redirect", fmt.Sprintf("/comm/%s/conf/%s/manage", comm.Alias, ctxt.GetScratch("currentAlias")), nil
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

/* FreezeTopic freezes or unfreezes the current topic.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func FreezeTopic(ctxt ui.AmContext) (string, any, error) {
	conf := ctxt.GetScratch("currentConference").(*database.Conference)
	myLevel := ctxt.GetScratch("levelInConference").(uint16)
	topic := ctxt.GetScratch("currentTopic").(*database.Topic)
	if !conf.TestPermission("Conference.Hide", myLevel) {
		ctxt.SetRC(http.StatusForbidden)
		return ui.ErrorPage(ctxt, ENOPERM)
	}
	err := topic.SetFrozen(ctxt.Ctx(), !topic.Frozen, ctxt.CurrentUser(), ctxt.RemoteIP())
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	return "redirect", fmt.Sprintf("/comm/%s/conf/%s/r/%d", ctxt.CurrentCommunity().Alias, ctxt.GetScratch("currentAlias"), topic.Number), nil
}

/* ArchiveTopic archives or unarchives the current topic.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func ArchiveTopic(ctxt ui.AmContext) (string, any, error) {
	conf := ctxt.GetScratch("currentConference").(*database.Conference)
	myLevel := ctxt.GetScratch("levelInConference").(uint16)
	topic := ctxt.GetScratch("currentTopic").(*database.Topic)
	if !conf.TestPermission("Conference.Hide", myLevel) {
		ctxt.SetRC(http.StatusForbidden)
		return ui.ErrorPage(ctxt, ENOPERM)
	}
	err := topic.SetArchived(ctxt.Ctx(), !topic.Archived, ctxt.CurrentUser(), ctxt.RemoteIP())
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	return "redirect", fmt.Sprintf("/comm/%s/conf/%s/r/%d", ctxt.CurrentCommunity().Alias, ctxt.GetScratch("currentAlias"), topic.Number), nil
}

/* StickTopic sticks or unsticks the current topic.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func StickTopic(ctxt ui.AmContext) (string, any, error) {
	conf := ctxt.GetScratch("currentConference").(*database.Conference)
	myLevel := ctxt.GetScratch("levelInConference").(uint16)
	topic := ctxt.GetScratch("currentTopic").(*database.Topic)
	if !conf.TestPermission("Conference.Hide", myLevel) {
		ctxt.SetRC(http.StatusForbidden)
		return ui.ErrorPage(ctxt, ENOPERM)
	}
	err := topic.SetSticky(ctxt.Ctx(), !topic.Sticky, ctxt.CurrentUser(), ctxt.RemoteIP())
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	return "redirect", fmt.Sprintf("/comm/%s/conf/%s/r/%d", ctxt.CurrentCommunity().Alias, ctxt.GetScratch("currentAlias"), topic.Number), nil
}

/* DeleteTopic deletes the current topic.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func DeleteTopic(ctxt ui.AmContext) (string, any, error) {
	if ctxt.CurrentUser().IsAnon {
		ctxt.SetRC(http.StatusForbidden)
		return ui.ErrorPage(ctxt, ENOPERM)
	}
	conf := ctxt.GetScratch("currentConference").(*database.Conference)
	myLevel := ctxt.GetScratch("levelInConference").(uint16)
	topic := ctxt.GetScratch("currentTopic").(*database.Topic)
	if !conf.TestPermission("Conference.Nuke", myLevel) {
		ctxt.SetRC(http.StatusForbidden)
		return ui.ErrorPage(ctxt, ENOPERM)
	}

	// Load the message box, and, if we have a valid "yes," then perform the delete
	mbox, err := ui.AmLoadMessageBox("deleteTopic")
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	if mbox.Validate(ctxt, "yes") {
		err := topic.Delete(ctxt.Ctx(), ctxt.CurrentUser(), ctxt.RemoteIP(), ampool)
		if err != nil {
			return ui.ErrorPage(ctxt, err)
		}
		return "redirect", fmt.Sprintf("/comm/%s/conf/%s", ctxt.CurrentCommunity().Alias, ctxt.GetScratch("currentAlias")), nil
	}

	// Set up to display the message box.
	mbox.SetMessage(fmt.Sprintf(`You are about to detele the topic <span class="font-bold text-red-600">"%s"</span>
		from the <span class="font-bold text-red-600">"%s"</span> conference!`, topic.Name, conf.Name))
	mbox.SetLink("no", fmt.Sprintf("/comm/%s/conf/%s/r/%d", ctxt.CurrentCommunity().Alias, ctxt.GetScratch("currentAlias"), topic.Number))
	mbox.SetLink("yes", fmt.Sprintf("/comm/%s/conf/%s/op/%d/delete", ctxt.CurrentCommunity().Alias, ctxt.GetScratch("currentAlias"), topic.Number))
	return mbox.Render(ctxt)
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

/* MoveMessageForm displays the form for moving a message..
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func MoveMessageForm(ctxt ui.AmContext) (string, any, error) {
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
	if !conf.TestPermission("Conference.Nuke", myLevel) || !conf.TestPermission("Conference.Post", myLevel) || topic.TopMessage == 0 {
		ctxt.SetRC(http.StatusForbidden)
		return ui.ErrorPage(ctxt, ENOPERM)
	}

	creator, err := hdrs[0].Creator(ctxt.Ctx())
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	ctxt.VarMap().Set("creator", creator)

	topiclist, err := database.AmListTopics(ctxt.Ctx(), conf.ConfId, ctxt.CurrentUserId(), database.TopicViewAll, database.TopicSortName, true)
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	for i := range topiclist {
		if topiclist[i].TopicID == topic.TopicId {
			topiclist = append(topiclist[:i], topiclist[i+1:]...)
			break
		}
	}
	ctxt.VarMap().Set("topiclist", topiclist)

	ctxt.VarMap().Set("post", hdrs[0])
	ctxt.VarMap().Set("topMessage", topic.TopMessage)
	formLink := fmt.Sprintf("/comm/%s/conf/%s/op/%d/move/%d", ctxt.CurrentCommunity().Alias, ctxt.GetScratch("currentAlias"), topic.Number, hdrs[0].Num)
	ctxt.VarMap().Set("formLink", formLink)
	ctxt.VarMap().Set("amsterdam_pageTitle", "Move Message")

	return "framed_template", "move_message.jet", nil
}

/* PublishMessage publishes a message to the front page.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func PublishMessage(ctxt ui.AmContext) (string, any, error) {
	if !ctxt.TestPermission("Global.PublishFP") {
		ctxt.SetRC(http.StatusForbidden)
		return ui.ErrorPage(ctxt, ENOPERM)
	}
	comm := ctxt.CurrentCommunity()
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

	if err = hdrs[0].Publish(ctxt.Ctx(), comm, ctxt.CurrentUser(), ctxt.RemoteIP()); err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	return "redirect", fmt.Sprintf("/comm/%s/conf/%s/r/%d?r=%d&ac=1", ctxt.CurrentCommunity().Alias, ctxt.GetScratch("currentAlias"), topic.Number, hdrs[0].Num), nil
}

/* MoveMessage moves a message to a different topic.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func MoveMessage(ctxt ui.AmContext) (string, any, error) {
	if ctxt.CurrentUser().IsAnon {
		ctxt.SetRC(http.StatusForbidden)
		return ui.ErrorPage(ctxt, ENOPERM)
	}
	comm := ctxt.CurrentCommunity()
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
	if ctxt.FormFieldIsSet("cancel") {
		return "redirect", fmt.Sprintf("/comm/%s/conf/%s/r/%d?r=%d&ac=1", comm.Alias, ctxt.GetScratch("currentAlias"), topic.Number, hdrs[0].Num), nil
	}
	if !conf.TestPermission("Conference.Nuke", myLevel) || !conf.TestPermission("Conference.Post", myLevel) || topic.TopMessage == 0 {
		ctxt.SetRC(http.StatusForbidden)
		return ui.ErrorPage(ctxt, ENOPERM)
	}
	targetId, err := ctxt.FormFieldInt("target")
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	target, err := database.AmGetTopic(ctxt.Ctx(), int32(targetId))
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}

	// Move the topic!
	err = hdrs[0].MoveTo(ctxt.Ctx(), target, ctxt.CurrentUser(), ctxt.RemoteIP())
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}

	// Now, we need to send this post to whoever subscribed to the NEW topic. But it's tricky because we don't have
	// all the information that we'd have if the post was just posted. Spool off any database operations in the task function.
	subs, err := target.GetSubscribers(ctxt.Ctx())
	if err != nil {
		log.Errorf("unable to deliver message to subscribers: %v", err)
	} else if len(subs) > 0 {
		// kick off a task to compose E-mails and deliver them to everyone
		alias := ctxt.GetScratch("currentAlias").(string)
		ipaddr := ctxt.RemoteIP()
		poster := ctxt.CurrentUser() // N.B.: only used for E-mail headers
		hdr := hdrs[0]
		ampool.Submit(func(ctx context.Context) {
			var postText string
			postText, err = hdr.Text(ctx)
			if err == nil {
				email.AmDeliverSubscription(ctx, comm, conf, alias, target, poster, hdr, postText, subs, ipaddr)
			} else {
				log.Errorf("unable to start AmDeliverSubscription - %v", err)
			}
		})
	}

	return "redirect", fmt.Sprintf("/comm/%s/conf/%s/r/%d", ctxt.CurrentCommunity().Alias, ctxt.GetScratch("currentAlias"), topic.Number), nil
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

	// Get the E-mail subscription status.
	sub, err := topic.IsSubscribed(ctxt.Ctx(), ctxt.CurrentUser())
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	ctxt.VarMap().Set("subscribed", sub)

	// Get the filtered users list.
	bozos, err := topic.GetBozos(ctxt.Ctx(), ctxt.CurrentUser())
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	ctxt.VarMap().Set("bozos", bozos)

	ctxt.VarMap().Set("amsterdam_pageTitle", "Manage Topic: "+topic.Name)
	return "framed_template", "manage_topic.jet", nil
}

/* TopicSetSubscribe toggles the "subscription" flag on the current topic for the current user.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func TopicSetSubscribe(ctxt ui.AmContext) (string, any, error) {
	if ctxt.CurrentUser().IsAnon {
		ctxt.SetRC(http.StatusForbidden)
		return ui.ErrorPage(ctxt, ENOPERM)
	}
	comm := ctxt.CurrentCommunity()
	topic := ctxt.GetScratch("currentTopic").(*database.Topic)
	flag, err := topic.IsSubscribed(ctxt.Ctx(), ctxt.CurrentUser())
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	err = topic.SetSubscribed(ctxt.Ctx(), ctxt.CurrentUser(), !flag)
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	return "redirect", fmt.Sprintf("/comm/%s/conf/%s/op/%d/manage", comm.Alias, ctxt.GetScratch("currentAlias"), topic.Number), nil
}

/* TopicRemoveBozo removes filtering from a specified user in the topic.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func TopicRemoveBozo(ctxt ui.AmContext) (string, any, error) {
	comm := ctxt.CurrentCommunity()
	topic := ctxt.GetScratch("currentTopic").(*database.Topic)
	bozoUid, err := strconv.Atoi(ctxt.URLParam("uid"))
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	err = topic.SetBozo(ctxt.Ctx(), ctxt.CurrentUser(), int32(bozoUid), false)
	if err != nil {
		return ui.ErrorPage(ctxt, err)
	}
	return "redirect", fmt.Sprintf("/comm/%s/conf/%s/op/%d/manage", comm.Alias, ctxt.GetScratch("currentAlias"), topic.Number), nil
}

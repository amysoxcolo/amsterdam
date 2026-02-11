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
	"errors"
	"fmt"
	"net/mail"

	"git.erbosoft.com/amy/amsterdam/database"
	"git.erbosoft.com/amy/amsterdam/email"
	"git.erbosoft.com/amy/amsterdam/ui"
)

/* InviteToCommunity displays the community invitation form.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func InviteToCommunity(ctxt ui.AmContext) (string, any) {
	if ctxt.CurrentUser().IsAnon {
		return "error", ENOPERM
	}
	comm := ctxt.CurrentCommunity()

	ctxt.SetFrameTitle("Send Invitation")
	ctxt.VarMap().Set("title", "Send Community Invitation")
	ctxt.VarMap().Set("subtitle", comm.Name)
	ctxt.VarMap().Set("backlink", fmt.Sprintf("/comm/%s/profile", comm.Alias))
	ctxt.VarMap().Set("cid", fmt.Sprintf("%d", comm.Id))
	return "framed", "invite.jet"
}

/* InviteToConference displays the conference invitation form.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func InviteToConference(ctxt ui.AmContext) (string, any) {
	if ctxt.CurrentUser().IsAnon {
		return "error", ENOPERM
	}
	comm := ctxt.CurrentCommunity()
	conf := ctxt.GetScratch("currentConference").(*database.Conference)

	ctxt.SetFrameTitle("Send Invitation")
	ctxt.VarMap().Set("title", "Send Conference Invitation")
	ctxt.VarMap().Set("subtitle", conf.Name)
	ctxt.VarMap().Set("backlink", fmt.Sprintf("/comm/%s/conf/%s/manage", comm.Alias, ctxt.GetScratch("currentAlias")))
	ctxt.VarMap().Set("cid", fmt.Sprintf("%d", comm.Id))
	ctxt.VarMap().Set("confid", fmt.Sprintf("%d", conf.ConfId))
	return "framed", "invite.jet"
}

/* InviteToTopic displays the topic invitation form.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func InviteToTopic(ctxt ui.AmContext) (string, any) {
	if ctxt.CurrentUser().IsAnon {
		return "error", ENOPERM
	}
	comm := ctxt.CurrentCommunity()
	conf := ctxt.GetScratch("currentConference").(*database.Conference)
	topic := ctxt.GetScratch("currentTopic").(*database.Topic)

	ctxt.SetFrameTitle("Send Invitation")
	ctxt.VarMap().Set("title", "Send Topic Invitation")
	ctxt.VarMap().Set("subtitle", topic.Name)
	ctxt.VarMap().Set("backlink", fmt.Sprintf("/comm/%s/conf/%s/op/%d/manage", comm.Alias, ctxt.GetScratch("currentAlias"), topic.Number))
	ctxt.VarMap().Set("cid", fmt.Sprintf("%d", comm.Id))
	ctxt.VarMap().Set("confid", fmt.Sprintf("%d", conf.ConfId))
	ctxt.VarMap().Set("topicid", fmt.Sprintf("%d", topic.TopicId))
	return "framed", "invite.jet"
}

/* InviteSend is the back end that handles sending invitations.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 */
func InviteSend(ctxt ui.AmContext) (string, any) {
	backlink := ctxt.FormField("backlink")
	if ctxt.FormFieldIsSet("cancel") {
		return "redirect", backlink
	} else if !ctxt.FormFieldIsSet("send") {
		return "error", EBUTTON
	}
	var comm *database.Community
	if ctxt.FormFieldIsSet("cid") {
		id, err := ctxt.FormFieldInt("cid")
		if err == nil {
			comm, err = database.AmGetCommunity(ctxt.Ctx(), int32(id))
		}
		if err != nil {
			return "error", err
		}
	} else {
		return "error", EPARAM
	}
	mode := "community"
	var conf *database.Conference = nil
	var topic *database.Topic = nil
	if ctxt.FormFieldIsSet("confid") {
		id, err := ctxt.FormFieldInt("confid")
		if err == nil {
			if conf, err = database.AmGetConference(ctxt.Ctx(), int32(id)); err == nil {
				var f bool
				if f, err = conf.InCommunity(ctxt.Ctx(), comm); err == nil {
					if !f {
						err = errors.New("invalid conference; not in community")
					}
				}
			}
		}
		if err != nil {
			return "errors", err
		}
		if ctxt.FormFieldIsSet("topicid") {
			id, err := ctxt.FormFieldInt("topicid")
			if err == nil {
				topic, err = database.AmGetTopic(ctxt.Ctx(), int32(id))
				if err == nil && topic.ConfId != conf.ConfId {
					err = errors.New("invalid topic; not in conference")
				}
			}
			if err != nil {
				return "errors", err
			}
			mode = "topic"
		} else {
			mode = "conference"
		}
	}
	addr := ctxt.FormField("addr")
	_, err := mail.ParseAddress(addr)
	if err != nil {
		return "errors", err
	}

	ci, err := database.AmGetContactInfoForUser(ctxt.Ctx(), ctxt.CurrentUserId())
	if err != nil {
		return "errors", err
	}

	mailMessage := email.AmNewEmailMessage(ctxt.CurrentUserId(), ctxt.RemoteIP())
	if comm.Public() {
		mailMessage.SetTemplate("invite_public.jet")
	} else {
		mailMessage.SetTemplate("invite_private.jet")
	}
	mailMessage.AddTo(addr, "")
	mailMessage.AddVariable("comm", comm)
	mailMessage.AddVariable("conf", conf)
	mailMessage.AddVariable("topic", topic)
	mailMessage.AddVariable("mode", mode)
	mailMessage.AddVariable("personal", ctxt.FormField("msg"))
	mailMessage.AddVariable("fullname", ci.FullName(true))
	mailMessage.AddVariable("username", ctxt.CurrentUser().Username)
	mailMessage.Send()

	return "redirect", backlink
}

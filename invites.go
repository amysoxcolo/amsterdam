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

	"git.erbosoft.com/amy/amsterdam/database"
	"git.erbosoft.com/amy/amsterdam/ui"
)

/* InviteToCommunity displays the community invitation form.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func InviteToCommunity(ctxt ui.AmContext) (string, any, error) {
	if ctxt.CurrentUser().IsAnon {
		ctxt.SetRC(http.StatusForbidden)
		return ui.ErrorPage(ctxt, ENOPERM)
	}
	comm := ctxt.CurrentCommunity()

	ctxt.VarMap().Set("amsterdam_pageTitle", "Send Invitation")
	ctxt.VarMap().Set("title", "Send Community Invitation")
	ctxt.VarMap().Set("subtitle", comm.Name)
	ctxt.VarMap().Set("backlink", fmt.Sprintf("/comm/%s/profile", comm.Alias))
	ctxt.VarMap().Set("cid", fmt.Sprintf("%d", comm.Id))
	return "framed_template", "invite.jet", nil
}

/* InviteToConference displays the conference invitation form.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func InviteToConference(ctxt ui.AmContext) (string, any, error) {
	if ctxt.CurrentUser().IsAnon {
		ctxt.SetRC(http.StatusForbidden)
		return ui.ErrorPage(ctxt, ENOPERM)
	}
	comm := ctxt.CurrentCommunity()
	conf := ctxt.GetScratch("currentConference").(*database.Conference)

	ctxt.VarMap().Set("amsterdam_pageTitle", "Send Invitation")
	ctxt.VarMap().Set("title", "Send Conference Invitation")
	ctxt.VarMap().Set("subtitle", conf.Name)
	ctxt.VarMap().Set("backlink", fmt.Sprintf("/comm/%s/conf/%s", comm.Alias, ctxt.GetScratch("currentAlias")))
	ctxt.VarMap().Set("cid", fmt.Sprintf("%d", comm.Id))
	ctxt.VarMap().Set("confid", fmt.Sprintf("%d", conf.ConfId))
	return "framed_template", "invite.jet", nil
}

/* InviteToTopic displays the topic invitation form.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func InviteToTopic(ctxt ui.AmContext) (string, any, error) {
	if ctxt.CurrentUser().IsAnon {
		ctxt.SetRC(http.StatusForbidden)
		return ui.ErrorPage(ctxt, ENOPERM)
	}
	comm := ctxt.CurrentCommunity()
	conf := ctxt.GetScratch("currentConference").(*database.Conference)
	topic := ctxt.GetScratch("currentTopic").(*database.Topic)

	ctxt.VarMap().Set("amsterdam_pageTitle", "Send Invitation")
	ctxt.VarMap().Set("title", "Send Topic Invitation")
	ctxt.VarMap().Set("subtitle", topic.Name)
	ctxt.VarMap().Set("backlink", fmt.Sprintf("/comm/%s/conf/%s/r/%d", comm.Alias, ctxt.GetScratch("currentAlias"), topic.Number))
	ctxt.VarMap().Set("cid", fmt.Sprintf("%d", comm.Id))
	ctxt.VarMap().Set("confid", fmt.Sprintf("%d", conf.ConfId))
	ctxt.VarMap().Set("topicid", fmt.Sprintf("%d", topic.TopicId))
	return "framed_template", "invite.jet", nil
}

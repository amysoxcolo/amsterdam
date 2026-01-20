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

	"git.erbosoft.com/amy/amsterdam/database"
	"git.erbosoft.com/amy/amsterdam/ui"
)

/* HideTopic hides or shows rthe current topic for the current user.
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

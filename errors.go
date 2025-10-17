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
	"git.erbosoft.com/amy/amsterdam/ui"
)

/* NotImplPage is used for all TODO links, to show that something hasn't yet been implemented.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func NotImplPage(ctxt ui.AmContext) (string, any, error) {
	ctxt.SetLeftMenu("top")
	ctxt.VarMap().Set("amsterdam_pageTitle", "Function Not Implemented")
	ctxt.VarMap().Set("path", ctxt.URLPath())
	return "framed_template", "notimpl.jet", nil
}

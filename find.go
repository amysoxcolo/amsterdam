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

import "git.erbosoft.com/amy/amsterdam/ui"

/* FindPage renders the Amsterdam "Find" page.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func FindPage(ctxt ui.AmContext) (string, any, error) {
	mode := ""
	p := ctxt.Parameter("mode")
	if p != "" {
		mode = p
	} else if ctxt.IsSession("find.mode") {
		x := ctxt.GetSession("find.mode")
		if xx, ok := x.(string); ok {
			mode = xx
		}
	}
	if mode == "" {
		mode = "COM"
	}
	switch mode {
	case "COM":
		ctxt.VarMap().Set("field", "name")
		ctxt.VarMap().Set("oper", "st")
		ctxt.VarMap().Set("term", "")
	case "USR":
		ctxt.VarMap().Set("field", "name")
		ctxt.VarMap().Set("oper", "st")
		ctxt.VarMap().Set("term", "")
	case "CAT":
		ctxt.VarMap().Set("oper", "st")
		ctxt.VarMap().Set("term", "")
	case "PST":
		ctxt.VarMap().Set("term", "")
	}

	ctxt.VarMap().Set("mode", mode)
	ctxt.VarMap().Set("amsterdam_pageTitle", "Find")
	ctxt.SetLeftMenu("top")
	ctxt.SetSession("find.mode", mode)
	return "framed_template", "find.jet", nil
}

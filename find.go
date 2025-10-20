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
	"strconv"

	"git.erbosoft.com/amy/amsterdam/database"
	"git.erbosoft.com/amy/amsterdam/ui"
)

// loadCategoryInformation loads the current category information to the context.
func loadCategoryInformation(ctxt ui.AmContext) error {
	u := ctxt.CurrentUser()
	catid := int32(-1)
	p := ctxt.Parameter("catid")
	if p != "" {
		v, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		catid = int32(v)
	} else if ctxt.IsSession("find.catid") {
		x := ctxt.GetSession("find.catid")
		if xx, ok := x.(int32); ok {
			catid = xx
		}
	}
	if catid > -1 {
		cat, err := database.AmGetCategory(catid) // this step also resolves symlinks
		if err != nil {
			return err
		}
		catid = cat.CatId
	}
	ctxt.SetSession("find.catid", catid)
	ctxt.VarMap().Set("catid", catid)
	ctxt.VarMap().Set("showHiddenCat", database.AmTestPermission("Global.ShowHiddenCategories", u.BaseLevel))
	hier, err := database.AmGetCategoryHierarchy(catid)
	if err != nil {
		return err
	}
	ctxt.VarMap().Set("catHierarchy", hier)
	subs, err := database.AmGetSubCategories(catid)
	if err != nil {
		return err
	}
	ctxt.VarMap().Set("catSubs", subs)
	// TODO: set matching communities as well
	return nil
}

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
	ctxt.SetSession("find.mode", mode)
	ctxt.VarMap().Set("mode", mode)
	switch mode {
	case "COM":
		ctxt.VarMap().Set("field", "name")
		ctxt.VarMap().Set("oper", "st")
		ctxt.VarMap().Set("term", "")
		err := loadCategoryInformation(ctxt)
		if err != nil {
			return ui.ErrorPage(ctxt, err)
		}
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

	ctxt.VarMap().Set("amsterdam_pageTitle", "Find")
	ctxt.SetLeftMenu("top")
	return "framed_template", "find.jet", nil
}

/*
 * Amsterdam Web Communities System
 * Copyright (c) 2025 Erbosoft Metaverse Design Solutions, All Rights Reserved
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */
package main

import (
	"fmt"

	"git.erbosoft.com/amy/amsterdam/ui"
	"github.com/labstack/echo/v4"
)

func setupEcho() *echo.Echo {
	e := echo.New()
	e.Renderer = &ui.TemplateRenderer{}
	e.GET("/img/*", ui.AmWrap(func(ctxt ui.AmContext) (string, any, error) {
		return "string", fmt.Sprintf("Path: %s", ctxt.URLPath()), nil
	}))
	e.GET("/", ui.AmWrap(func(ctxt ui.AmContext) (string, any, error) {
		ctxt.VarMap().Set("amsterdam_pageTitle", "My Front Page")
		return "framed_template", "top.jet", nil
	}))
	return e
}

func main() {
	e := setupEcho()
	e.Logger.Fatal(e.Start(":1323"))
}

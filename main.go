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
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// setupEcho creates, configures, and returns a new Echo instance.
func setupEcho() *echo.Echo {
	e := echo.New()
	e.Logger = &EchoLogrusAdapter{}
	e.Renderer = &ui.TemplateRenderer{}
	e.Use(middleware.Recover())
	e.Use(LogrusMiddleware)

	e.GET("/img/*", ui.AmWrap(ui.AmServeImage))
	e.GET("/", ui.AmWrap(func(ctxt ui.AmContext) (string, any, error) {
		ctxt.VarMap().Set("amsterdam_pageTitle", "My Front Page")
		return "framed_template", "top.jet", nil
	}))

	return e
}

// main is Ye Olde Main Function.
func main() {
	e := setupEcho()

	e.Logger.Fatal(e.Start(":1323"))
}

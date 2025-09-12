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
	"io"
	"net/http"

	"github.com/CloudyKit/jet/v6"
	"github.com/labstack/echo/v4"
)

var views = jet.NewSet(
	jet.NewOSFileSystemLoader("./views"),
	jet.DevelopmentMode(true),
)

type TemplateRenderer struct {
}

func (self *TemplateRenderer) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	view, err := views.GetTemplate(name)
	if err != nil {
		return err
	}
	return view.Execute(w, nil, nil)
}

func setupEcho() *echo.Echo {
	e := echo.New()
	e.Renderer = &TemplateRenderer{}
	e.GET("/", func(c echo.Context) error {
		return c.Render(http.StatusOK, "frame.jet", nil)
	})
	return e
}

func main() {
	e := setupEcho()
	e.Logger.Fatal(e.Start(":1323"))
}

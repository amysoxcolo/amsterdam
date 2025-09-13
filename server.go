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

	"git.erbosoft.com/amy/amsterdam/ui"
	"github.com/CloudyKit/jet/v6"
	"github.com/labstack/echo/v4"
)

var views = jet.NewSet(
	jet.NewOSFileSystemLoader("./views"),
	jet.DevelopmentMode(true),
)

type TemplateRenderer struct {
}

func (r *TemplateRenderer) Render(w io.Writer, name string, data any, c echo.Context) error {
	view, err := views.GetTemplate(name)
	if err != nil {
		return err
	}
	return view.Execute(w, nil, nil)
}

func setupEcho() *echo.Echo {
	e := echo.New()
	e.Renderer = &TemplateRenderer{}
	e.GET("/", ui.AmWrap(func(ctxt ui.AmContext) (string, any, error) {
		return "template", "frame.jet", nil
	}))
	return e
}

func main() {
	e := setupEcho()
	e.Logger.Fatal(e.Start(":1323"))
}

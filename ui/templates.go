/*
 * Amsterdam Web Communities System
 * Copyright (c) 2025 Erbosoft Metaverse Design Solutions, All Rights Reserved
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */
package ui

import (
	"io"

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
	var vmap jet.VarMap = nil
	amctxt := AmContextFromEchoContext(c)
	if amctxt != nil {
		vmap = amctxt.VarMap()
	}
	return view.Execute(w, vmap, data)
}

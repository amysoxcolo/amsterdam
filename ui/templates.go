/*
 * Amsterdam Web Communities System
 * Copyright (c) 2025 Erbosoft Metaverse Design Solutions, All Rights Reserved
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

// Package ui holds the support for the Amsterdam user interface, wrapping Echo and Jet templates.
package ui

import (
	"io"

	"git.erbosoft.com/amy/amsterdam/config"
	"github.com/CloudyKit/jet/v6"
	"github.com/labstack/echo/v4"
)

// views is the main Jet template repository.
var views = jet.NewSet(
	jet.NewOSFileSystemLoader("./views"),
	jet.DevelopmentMode(true),
)

// init adds additional configuration for the views object.
func init() {
	views.AddGlobal("GlobalConfig", config.GlobalConfig)
}

// TemplateRenderer is the Renderer instance set into the Echo context at creation time, to render Jet templates.
type TemplateRenderer struct{}

/* Render renders a Jet template to the Echo output stream.
 * Parameters:
 *     w - Echo's output stream writer.
 *     name - Name of the template to be rendered.
 *     data - Context data to pass to the template.
 *     c - The Echo context for the request being processed.
 * Returns:
 *     Standard Go error status.
 */
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

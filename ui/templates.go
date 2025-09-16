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
	"embed"
	"fmt"
	"io"

	"git.erbosoft.com/amy/amsterdam/config"
	"github.com/CloudyKit/jet/v6"
	"github.com/labstack/echo/v4"
)

//go:embed views/*
var static_views embed.FS

// EmbeddedLoader is our implementation of Loader that references an embedded filesystem.
type EmbeddedLoader struct {
	efs    embed.FS
	prefix string
}

/* Exists (implements Loader) tests if a particular template exists.
 * Parameters:
 *     templatePath - Path of the template to be tested.
 * Returns:
 *     true if the template exists, false if not.
 */
func (l *EmbeddedLoader) Exists(templatePath string) bool {
	file, err := l.efs.Open(fmt.Sprintf("%s%s", l.prefix, templatePath))
	if err == nil {
		file.Close()
		return true
	}
	return false
}

/* Open (implements Loader) opens a template file.
 * Parameters:
 *     templatePath - Path of the template to open.
 * Returns:
 *     Handle to the opened template file
 *.    Standard Go error status.
 */
func (l *EmbeddedLoader) Open(templatePath string) (io.ReadCloser, error) {
	return l.efs.Open((fmt.Sprintf("%s%s", l.prefix, templatePath)))
}

// views is the main Jet template repository.
var views = jet.NewSet(
	&EmbeddedLoader{efs: static_views, prefix: "views"},
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

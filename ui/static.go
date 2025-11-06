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
	"io/fs"
	"net/http"

	"github.com/labstack/echo/v4"
)

//go:embed static/*
var static_data embed.FS

// AmStaticFileHandler returns a handler for the files in the static embedded filesystem.
func AmStaticFileHandler() echo.HandlerFunc {
	fsys, err := fs.Sub(static_data, "static")
	if err != nil {
		panic(err)
	}
	return echo.WrapHandler(http.StripPrefix("/static/", http.FileServer(http.FS(fsys))))
}

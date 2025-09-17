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
	"mime"
	"net/http"
	"path/filepath"
	"strings"
)

//go:embed static_images/*
var static_images embed.FS

/* mimeTypeFromFilenane returns the MIME type of a file, given its filename.
 * Parameters:
 *     filaname - The name of the file to be tested.
 * Returns:
 *     The file's inferred MIME type.
 */
func mimeTypeFromFilename(filename string) string {
	return mime.TypeByExtension(filename[strings.LastIndex(filename, "."):])
}

/* AmServeImage serves an image from internal storage.
 * Parameters:
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Type of content to be rendered
 *     Content to be rendered
 *     Standard Go error return
 */
func AmServeImage(ctxt AmContext) (string, any, error) {
	components := strings.SplitAfter(ctxt.URLPath(), "/")
	var err error = nil
	if len(components) == 4 && components[2] == "builtin/" {
		var b []byte
		b, err = static_images.ReadFile(filepath.Join("static_images", components[3]))
		if err == nil {
			ctxt.SetOutputType(mimeTypeFromFilename(components[3]))
			return "bytes", b, nil
		}
	}
	ctxt.SetRC(http.StatusNotFound)
	// TODO: improve this error reporting
	return "string", fmt.Sprintf("File not found: %s", ctxt.URLPath()), err
}

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
	"embed"
	"fmt"
	"mime"
	"net/http"
	"strings"
)

//go:embed static_images/*
var static_images embed.FS

func mimeTypeFromFilename(filename string) string {
	return mime.TypeByExtension(filename[strings.LastIndex(filename, "."):])
}

func AmServeImage(ctxt AmContext) (string, any, error) {
	components := strings.SplitAfter(ctxt.URLPath(), "/")
	var err error = nil
	if len(components) == 4 && components[2] == "builtin/" {
		var b []byte
		b, err = static_images.ReadFile(fmt.Sprintf("static_images/%s", components[3]))
		if err == nil {
			ctxt.SetOutputType(mimeTypeFromFilename(components[3]))
			return "bytes", b, nil
		}
	}
	ctxt.SetRC(http.StatusNotFound)
	return "string", fmt.Sprintf("File not found: %s", ctxt.URLPath()), err
}

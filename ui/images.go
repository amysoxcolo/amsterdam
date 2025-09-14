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

func AmServeImage(ctxt AmContext) (string, any, error) {
	components := strings.SplitAfter(ctxt.URLPath(), "/")
	var err error = nil
	if len(components) == 2 && components[0] == "builtin" {
		var b []byte
		b, err = static_images.ReadFile(fmt.Sprintf("static_images/%s", components[1]))
		if err == nil {
			mtype := mime.TypeByExtension(components[1][strings.LastIndex(components[1], "."):])
			ctxt.SetOutputType(mtype)
			return "bytes", b, nil
		}
	}
	ctxt.SetRC(http.StatusNotFound)
	return "string", fmt.Sprintf("File not found: %s", ctxt.URLPath()), err
}

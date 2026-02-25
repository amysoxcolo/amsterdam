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
	"bytes"
	"embed"
	"io"
	"io/fs"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"golang.org/x/net/html"
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

// extractPlainText extracts all plain text from a HTML tree node.
func extractPlainText(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			sb.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return sb.String()
}

// extractInnerHTML extracts the inner HTML from a HTML tree node.
func extractInnerHTML(n *html.Node) string {
	var buf bytes.Buffer
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		html.Render(&buf, c)
	}
	return buf.String()
}

// breakUpHTML extracts the title and body from an HTML page.
func breakUpHTML(r io.Reader) (string, string, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return "", "", err
	}
	title := ""
	body := ""
	var traverse func(*html.Node)
	traverse = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "title":
				body = extractPlainText(n)
			case "body":
				body = extractInnerHTML(n)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}
	}
	traverse(doc)
	return title, body, nil
}

/* AmStaticFramePage generates a handler that will serve up data from an external filesystem "framed" inside
 * the frame with the template engine.
 * Parameters:
 *     staticFS - The filesystem to serve from.
 *     prefix - The prefix to be stripped off pathnames to feed to the filesystem.
 * Returns:
 *     An AmPageFunc suitable for wrapping and adding to the Echo handlers.
 */
func AmStaticFramePage(staticFS fs.FS, prefix string) AmPageFunc {
	return func(ctxt AmContext) (string, any) {
		fname := ctxt.URLPath()
		if strings.HasPrefix(fname, prefix) {
			fname = fname[len(prefix):]
		} else {
			return "error", "invalid path name"
		}
		mtype := mimeTypeFromFilename(fname)
		ctxt.VarMap().Set("mimeType", mtype)
		switch mtype {
		case "text/html":
			f, err := staticFS.Open(fname)
			if err != nil {
				return "error", err
			}
			defer f.Close()
			title, body, err := breakUpHTML(f)
			if err != nil {
				return "error", err
			}
			ctxt.SetFrameTitle(title)
			ctxt.VarMap().Set("title", title)
			ctxt.VarMap().Set("data", body)
			return "framed", "extern.jet"
		}
		return "error", "Unknown MIME Type: " + mtype
	}
}

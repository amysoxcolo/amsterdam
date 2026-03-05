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
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"strings"

	"git.erbosoft.com/amy/amsterdam/config"
	"github.com/labstack/echo/v4"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/html"
)

//go:embed static/*
var static_data embed.FS

//go:embed resources/*
var static_resources embed.FS

// external_resources is the link to the external resource path.
var external_resources fs.FS = nil

func setupResources() {
	// Open the external resource path.
	if config.GlobalConfig.Resources.ExternalResourcePath != "" {
		finfo, err := os.Stat(config.GlobalConfig.Resources.ExternalResourcePath)
		if err == nil {
			if finfo.IsDir() {
				root, err := os.OpenRoot(config.GlobalConfig.Resources.ExternalResourcePath)
				if err != nil {
					panic(err)
				}
				external_resources = root.FS()
			} else {
				log.Errorf("external resource path \"%s\" is not a directory, ignored", config.GlobalConfig.Resources.ExternalResourcePath)
			}
		} else {
			log.Errorf("external resource path \"%s\" is not valid, ignored (%v)", config.GlobalConfig.Resources.ExternalResourcePath, err)
		}
	}
}

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
				title = extractPlainText(n)
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

// AmLoadHTMLResource loads an HTML resource and splits it into title and body.
func AmLoadHTMLResource(resourceName string) (string, string, error) {
	var f fs.File = nil
	var err error
	if external_resources != nil {
		f, err = external_resources.Open(resourceName)
		if err != nil {
			f = nil
			pe := err.(*fs.PathError)
			if pe.Err == os.ErrInvalid || pe.Err == os.ErrNotExist {
				err = nil
			}
		}
		if err != nil {
			return "", "", err
		}
	}
	if f == nil {
		f, err = static_resources.Open(fmt.Sprintf("resources/%s", resourceName))
		if err != nil {
			return "", "", err
		}
	}
	title, body, err := breakUpHTML(f)
	f.Close()
	return title, body, err
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
		// Cut the prefix off the path.
		fname := ctxt.URLPath()
		if strings.HasPrefix(fname, prefix) {
			fname = fname[len(prefix):]
		} else {
			return "error", "invalid path name"
		}
		// Extract the basic MIME type.
		mtype := mimeTypeFromFilename(fname)
		p := strings.Index(mtype, ";")
		if p >= 0 {
			mtype = mtype[:p]
		}
		// Decide from there how to render it.
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

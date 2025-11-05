/*
 * Amsterdam Web Communities System
 * Copyright (c) 2025 Erbosoft Metaverse Design Solutions, All Rights Reserved
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */
// The htmlcheck package contains the HTML Checker.
package htmlcheck

import (
	"net/url"
	"regexp"
	"strings"
)

// urlElement is a basic element
type urlElement struct {
	re     *regexp.Regexp
	prefix string
}

// eval matches the argument against our regular expression and adds an optional prefix.
func (e *urlElement) eval(s string) string {
	if e.re.FindString(s) == "" {
		return ""
	}
	return e.prefix + s
}

// urlSetupData contains the data needed to set up the URL rewriter elements.
var urlSetupData = [...]string{
	`^(?i:http)://[A-Za-z0-9_-]+(\.[A-Za-z0-9_-]+)+.*$`, "",
	`^(?i:https)://[A-Za-z0-9_-]+(\.[A-Za-z0-9_-]+)+.*$`, "",
	`^(?i:ftp)://[A-Za-z0-9_-]+(\.[A-Za-z0-9_-]+)+.*$`, "",
	`^(?i:gopher)://[A-Za-z0-9_-]+(\.[A-Za-z0-9_-]+)+.*$`, "",
	"^(?i:mailto):[A-Za-z0-9!#$%*+-/=?^_`{|}~.]+@[A-Za-z0-9_-]+(\\.[A-Za-z0-9_-]+)+$", "",
	`^(?i:news):[A-Za-z0-9_-]+(\.[A-Za-z0-9_-]+)+$`, "",
	`^(?i:nntp)://[A-Za-z0-9_-]+(\.[A-Za-z0-9_-]+)+.*$`, "",
	`^(?i:telnet)://[A-Za-z0-9_-]+(\.[A-Za-z0-9_-]+)+.*$`, "",
	`^(?i:tn3270)://[A-Za-z0-9_-]+(\.[A-Za-z0-9_-]+)+.*$`, "",
	`^(?i:www)\.[A-Za-z0-9_-]+(\.[A-Za-z0-9_-]+)*.*$`, "http://",
	`^(?i:ftp)\.[A-Za-z0-9_-]+(\.[A-Za-z0-9_-]+)*.*$`, "ftp://",
	`^(?i:gopher)\.[A-Za-z0-9_-]+(\.[A-Za-z0-9_-]+)*.*$`, "gopher://",
}

// The URL elements we can match against.
var urlElements []urlElement

// init builds the URL elements and registers the rewriter.
func init() {
	urlElements = make([]urlElement, len(urlSetupData)/2)
	i, j := 0, 0
	for i < len(urlSetupData) {
		urlElements[j].re = regexp.MustCompile(urlSetupData[i])
		urlElements[j].prefix = urlSetupData[i+1]
		i += 2
		j++
	}
	r := urlRewriter{}
	rewriterRegistry[r.Name()] = &r
}

// urlRewriter is an implementation of Rewriter that recognizes URLs.
type urlRewriter struct{}

// Name returns the rewriter's name.
func (rw *urlRewriter) Name() string {
	return "url"
}

/* Rewrite rewrites the given string data and adds markup before and after if needed.
 * Parameters:
 *     data - The data to be rewritten.
 *     svc - Services interface we can use.
 * Returns:
 *     Pointer to markup data, or nil.
 */
func (rw *urlRewriter) Rewrite(data string, svc rewriterServices) *markupData {
	for _, ue := range urlElements {
		s := ue.eval(data)
		if s != "" {
			ls := strings.ToLower(s)
			if strings.HasPrefix(ls, "http:") || strings.HasPrefix(ls, "https:") {
				url, err := url.Parse(s)
				if err == nil {
					svc.addExternalRef(url)
				}
			}
			var openA strings.Builder
			openA.WriteString("<a href=\"")
			openA.WriteString(s)
			openA.WriteString("\"")
			catenate := svc.rewriterAttrValue("ANCHORTAIL")
			if catenate != "" {
				openA.WriteString(" ")
				openA.WriteString(catenate)
			}
			openA.WriteString(">")
			return &markupData{
				beginMarkup: openA.String(),
				text:        data,
				endMarkup:   "</a>",
				rescan:      false,
			}
		}
	}
	return nil
}

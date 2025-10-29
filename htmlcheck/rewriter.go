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
	"net/mail"
	"net/url"
	"strings"
)

// markupData holds the return from rewriters.
type markupData struct {
	beginMarkup string
	text        string
	endMarkup   string
	rescan      bool
}

// rewriterServices is an interface that provides services to rewriters.
type rewriterServices interface {
	rewriterAttrValue(string) string
	rewriterContextValue(string) any
	addExternalRef(*url.URL)
	addInternalRef(string)
}

// rewriter is the interface for components that rewrite source text and place markup around it.
type rewriter interface {
	Name() string
	Rewrite(string, rewriterServices) *markupData
}

// emailRewriter is an implementation of Rewriter that recognizes E-mail addresses.
type emailRewriter struct{}

// EmailRewriter is a singleton implementration of rewriter for E-mail addresses.
var EmailRewriter = emailRewriter{}

// Name returns the rewriter's name.
func (rw *emailRewriter) Name() string {
	return "email"
}

/* Rewrite rewrites the given string data and adds markup before and after if needed.
 * Parameters:
 *     data - The data to be rewritten.
 *     svc - Services interface we can use.
 * Returns:
 *     Pointer to markup data, or nil.
 */
func (rw *emailRewriter) Rewrite(data string, svc rewriterServices) *markupData {
	_, err := mail.ParseAddress(data)
	if err != nil {
		return nil
	}

	var openA strings.Builder
	openA.WriteString("<a href=\"mailto:")
	openA.WriteString(data)
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
		rescan:      false}
}

// urlRewriter is an implementation of Rewriter that recognizes URLs.
type urlRewriter struct{}

// URLRewriter is a singleton implementration of rewriter for URLs.
var URLRewriter = urlRewriter{}

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
	url, err := url.Parse(data)
	if err != nil {
		secondChance := ""
		if strings.HasPrefix(data, "www.") {
			secondChance = "http://" + data
		} else if strings.HasPrefix(data, "ftp.") {
			secondChance = "ftp://" + data
		} else if strings.HasPrefix(data, "gopher.") {
			secondChance = "gopher://" + data
		}
		if secondChance == "" {
			return nil
		}
		url, err = url.Parse(secondChance)
		if err != nil {
			return nil
		}
	}

	if url.Scheme == "http" || url.Scheme == "https" {
		svc.addExternalRef(url)
	}

	var openA strings.Builder
	openA.WriteString("<a href=\"")
	openA.WriteString(url.String())
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

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
	"fmt"
	"net/mail"
	"net/url"
	"strings"

	"git.erbosoft.com/amy/amsterdam/database"
)

// markupData holds the return from rewriters.
type markupData struct {
	beginMarkup string
	text        string
	endMarkup   string
	rescan      bool
}

func (md *markupData) all() string {
	return md.beginMarkup + md.text + md.endMarkup
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

// rewriterRegistry contains a list of all rewriters.
var rewriterRegistry = make(map[string]rewriter)

// init registers our rewriters with the registry.
func init() {
	r1 := emailRewriter{}
	rewriterRegistry[r1.Name()] = &r1
	r2 := postLinkRewriter{}
	rewriterRegistry[r2.Name()] = &r2
	r3 := userLinkRewriter{}
	rewriterRegistry[r3.Name()] = &r3
}

// emailRewriter is an implementation of Rewriter that recognizes E-mail addresses.
type emailRewriter struct{}

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

// postLinkRewriter is the rewriter that handles links to conference posts.
type postLinkRewriter struct{}

// postLinkURLPrefix is the default URL prefix for post links.
const postLinkURLPrefix = "x-postlink:"

// Name returns the rewriter's name.
func (rw *postLinkRewriter) Name() string {
	return "postlink"
}

// buildPostLink constructs a full post link from decoded data and context.
func buildPostLink(decoded, context *database.PostLinkData) string {
	var b strings.Builder
	started := false
	if decoded.Community == "" {
		b.WriteString(context.Community)
	} else {
		b.WriteString(decoded.Community)
		started = true
	}
	b.WriteString("!")
	if decoded.Conference == "" {
		if started {
			return b.String()
		}
		b.WriteString(context.Conference)
	} else {
		b.WriteString(decoded.Conference)
	}
	b.WriteString(".")
	if decoded.Topic == -1 {
		if started {
			return b.String()
		}
		b.WriteString(fmt.Sprintf("%d", context.Topic))
	} else {
		b.WriteString(fmt.Sprintf("%d", decoded.Topic))
	}
	b.WriteString(".")
	if decoded.FirstPost != -1 {
		b.WriteString(fmt.Sprintf("%d", decoded.FirstPost))
		if decoded.FirstPost != decoded.LastPost {
			b.WriteString("-")
			if decoded.LastPost != -1 {
				b.WriteString(fmt.Sprintf("%d", decoded.LastPost))
			}
		}
	}
	return b.String()
}

/* Rewrite rewrites the given string data and adds markup before and after if needed.
 * Parameters:
 *     data - The data to be rewritten.
 *     svc - Services interface we can use.
 * Returns:
 *     Pointer to markup data, or nil.
 */
func (rw *postLinkRewriter) Rewrite(data string, svc rewriterServices) *markupData {
	q := svc.rewriterContextValue("PostLinkDecoderContext")
	if q == nil {
		return nil
	}
	ctxt := q.(*database.PostLinkData)

	mydata, err := database.AmDecodePostLink(data)
	if err != nil {
		return nil
	}
	err = mydata.VerifyNames()
	if err != nil {
		return nil
	}
	// build post link, add it as an internal reference
	link := buildPostLink(mydata, ctxt)
	svc.addInternalRef(link)
	// build the necessary markup and return it
	var openA strings.Builder
	openA.WriteString("<a href=\"")
	openA.WriteString(postLinkURLPrefix)
	openA.WriteString(link)
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

// userLinkRewriter is the rewriter that handles links to user names.
type userLinkRewriter struct{}

// userLinkURIPrefix is the default URL prefix for user links.
const userLinkURIPRefix = "x-userlink:"

// Name returns the rewriter's name.
func (rw *userLinkRewriter) Name() string {
	return "userlink"
}

/* Rewrite rewrites the given string data and adds markup before and after if needed.
 * Parameters:
 *     data - The data to be rewritten.
 *     svc - Services interface we can use.
 * Returns:
 *     Pointer to markup data, or nil.
 */
func (rw *userLinkRewriter) Rewrite(data string, svc rewriterServices) *markupData {
	if data == "" || len(data) > 64 || !database.AmIsValidAmsterdamID(data) {
		return nil
	}

	user, err := database.AmGetUserByName(data)
	if err != nil || user == nil {
		return nil
	}

	// build the necessary markup and return it
	var openA strings.Builder
	openA.WriteString("<a href=\"")
	openA.WriteString(userLinkURIPRefix)
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
		rescan:      false,
	}
}

// countingRewriter is a wrapper around rewriter that counts the number of rewrites.
type countingRewriter struct {
	inner rewriter
	count int
}

// Name returns the rewriter's name.
func (rw *countingRewriter) Name() string {
	return rw.inner.Name()
}

/* Rewrite rewrites the given string data and adds markup before and after if needed.
 * Parameters:
 *     data - The data to be rewritten.
 *     svc - Services interface we can use.
 * Returns:
 *     Pointer to markup data, or nil.
 */
func (rw *countingRewriter) Rewrite(data string, svc rewriterServices) *markupData {
	rc := rw.inner.Rewrite(data, svc)
	if rc != nil && !rc.rescan {
		rw.count++
	}
	return rc
}

// GetCount returns the rewriter's count.
func (rw *countingRewriter) GetCount() int {
	return rw.count
}

// Reset resets the rewriter.
func (rw *countingRewriter) Reset() {
	rw.count = 0
}

// MakeCountingRewriter wraps the rewriter in a countingRewriter.
func MakeCountingRewriter(rw rewriter) *countingRewriter {
	return &countingRewriter{
		inner: rw,
		count: 0,
	}
}

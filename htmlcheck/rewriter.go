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

import "net/url"

type markupData struct {
	beginMarkup string
	text        string
	endMarkup   string
	rescan      bool
}

type rewriterServices interface {
	rewriterAttrValue(string) string
	rewriterContextValue(string) any
	addExternalRef(url.URL)
	addInternalRef(string)
}

type rewriter interface {
	Name() string
	Rewrite(string, rewriterServices) *markupData
}

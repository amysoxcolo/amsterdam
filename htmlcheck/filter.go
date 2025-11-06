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

import "strings"

// outputFilter is the interface for an HTML checker output filter.
type outputFilter interface {
	tryOutputRune(*strings.Builder, rune) bool
	matchRune(rune) bool
	lengthNoMatch(string) int
}

// outputFilterRegistry contains a listing of all defined output filters.
var outputFilterRegistry = make(map[string]outputFilter)

// init registers all known filters.
func init() {
	f := htmlEncodingFilter{}
	outputFilterRegistry["html"] = &f
}

// htmlEncodingFilter is a filter that escapes certain characters in HTML.
type htmlEncodingFilter struct{}

// htmlEscapedChars is a list of HTML characters that are escaped.
const htmlEscapedChars = "<>&"

// tryOutputRune outputs a rune that needs to be escaped.
func (f *htmlEncodingFilter) tryOutputRune(buf *strings.Builder, ch rune) bool {
	switch ch {
	case '<':
		buf.WriteString("&lt;")
	case '>':
		buf.WriteString("&gt;")
	case '&':
		buf.WriteString("&amp;")
	default:
		return false
	}
	return true
}

// matchCharacter returns true if this character needs to be escaped.
func (f *htmlEncodingFilter) matchRune(ch rune) bool {
	return strings.ContainsRune(htmlEscapedChars, ch)
}

// lengthNoMatch returns the maximum length of unmatched characters at the start of the string.
func (f *htmlEncodingFilter) lengthNoMatch(s string) int {
	rc := len(s)
	for _, c := range htmlEscapedChars {
		tmp := strings.IndexRune(s, c)
		if tmp >= 0 && tmp < rc {
			rc = tmp
			if rc == 0 {
				return 0
			}
		}
	}
	return rc
}

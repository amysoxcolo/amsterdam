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
	"net/url"
	"regexp"
	"strings"
)

type causeLineBreakFunc func(*tag, bool) bool
type closingTagFunc func(*tag) string
type rewriteContentsFunc func(*tag, string, bool, htmlCheckerBackend) string

type tag struct {
	name        string
	index       int
	lineBreak   bool
	allowClose  bool
	balanceTags bool
	clb         causeLineBreakFunc
	ct          closingTagFunc
	rwc         rewriteContentsFunc
}

func (t *tag) causeLineBreak(isClosing bool) bool {
	if t.clb == nil {
		return t.lineBreak
	}
	return t.clb(t, isClosing)
}

func (t *tag) makeClosingTag() string {
	if t.ct == nil {
		return ""
	}
	return t.ct(t)
}

func (t *tag) rewriteContents(contents string, isClosing bool, ctxt htmlCheckerBackend) string {
	if t.rwc == nil {
		return contents
	}
	return t.rwc(t, contents, isClosing, ctxt)
}

func createSimpleTag(name string, brk bool) *tag {
	return &tag{
		name:        strings.ToUpper(name),
		index:       -1,
		lineBreak:   brk,
		allowClose:  false,
		balanceTags: false,
		clb:         nil,
		ct:          nil,
		rwc:         nil,
	}
}

func createWBRTag() *tag {
	return &tag{
		name:        "WBR",
		index:       -1,
		lineBreak:   false,
		allowClose:  false,
		balanceTags: false,
		clb:         nil,
		ct:          nil,
		rwc: func(t *tag, contents string, isClosing bool, ctxt htmlCheckerBackend) string {
			ctxt.sendTagMessage("WBR")
			return contents
		},
	}
}

func stdClosingTag(tag *tag) string {
	return fmt.Sprintf("</%s>", tag.name)
}

func createOpenCloseTag(name string, brk bool) *tag {
	return &tag{
		name:        strings.ToUpper(name),
		index:       -1,
		lineBreak:   brk,
		allowClose:  true,
		balanceTags: false,
		clb:         nil,
		ct:          stdClosingTag,
		rwc:         nil,
	}
}

func createListElementTag(name string) *tag {
	return &tag{
		name:        strings.ToUpper(name),
		index:       -1,
		lineBreak:   true,
		allowClose:  true,
		balanceTags: false,
		clb: func(t *tag, isClosing bool) bool {
			return !isClosing
		},
		ct:  stdClosingTag,
		rwc: nil,
	}
}

func createBalancedTag(name string, brk bool) *tag {
	return &tag{
		name:        strings.ToUpper(name),
		index:       -1,
		lineBreak:   brk,
		allowClose:  true,
		balanceTags: true,
		clb:         nil,
		ct:          stdClosingTag,
		rwc:         nil,
	}
}

func createNOBRTag() *tag {
	return &tag{
		name:        "NOBR",
		index:       -1,
		lineBreak:   false,
		allowClose:  true,
		balanceTags: true,
		clb:         nil,
		ct:          stdClosingTag,
		rwc: func(t *tag, contents string, isClosing bool, ctxt htmlCheckerBackend) string {
			if isClosing {
				ctxt.sendTagMessage("/NOBR")
			} else {
				ctxt.sendTagMessage("NOBR")
			}
			return contents
		},
	}
}

var hrefPattern = regexp.MustCompile(`(?i:href\s*=)`)
var targetPattern = regexp.MustCompile(`(?i:target\s*=)`)

func extractAttribute(s string) string {
	s = strings.TrimSpace(s)
	if s[0] == '\'' || s[0] == '"' {
		p := strings.IndexByte(s[1:], s[0])
		if p < 0 {
			return ""
		}
		return s[1 : p+1]
	}
	return strings.Fields(s)[0]
}

func rewriteATagContents(t *tag, contents string, isClosing bool, ctxt htmlCheckerBackend) string {
	if isClosing {
		return contents // don't bother checking close tag
	}
	bounds := hrefPattern.FindStringIndex(contents)
	if bounds != nil {
		s := extractAttribute(contents[bounds[1]:])
		if s != "" && (strings.HasPrefix(s, "http:") || strings.HasPrefix(s, "https:")) {
			ref, err := url.Parse(s)
			if err != nil {
				ctxt.addExternalRef(ref)
			}
		}
	}

	targetSeen := false
	bounds = targetPattern.FindStringIndex(contents)
	if bounds != nil {
		s := extractAttribute(contents[bounds[1]:])
		if s != "" {
			targetSeen = true
		}
	}
	if targetSeen {
		return contents
	}
	tail := ctxt.getCheckerAttrValue("ANCHORTAIL")
	return contents + " " + tail
}

func createATag() *tag {
	return &tag{
		name:        "A",
		index:       -1,
		lineBreak:   false,
		allowClose:  true,
		balanceTags: true,
		clb:         nil,
		ct:          stdClosingTag,
		rwc:         rewriteATagContents,
	}
}

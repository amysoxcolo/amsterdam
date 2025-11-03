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

	"github.com/bits-and-blooms/bitset"
)

// Constants used to group individual HTML tags.
const (
	tagSetInlineFormat      = 1  // inline formatting
	tagSetAnchor            = 2  // the <A> tag
	tagSetBlockFormat       = 3  // block-level formatting
	tagSetActiveContent     = 4  // active content like objects and scripts
	tagSetImageMaps         = 5  // image map tags
	tagSetDocFormat         = 6  // document-level formatting
	tagSetFontFormat        = 7  // the <FONT> tag
	tagSetForms             = 8  // form tags
	tagSetTables            = 9  // table tags
	tagSetChangeMarkup      = 10 // change markup (<DEL> and <INS>)
	tagSetFrames            = 11 // frame tags
	tagSetImages            = 12 // the <IMG> tag
	tagSetPreformat         = 13 // the <PRE> tag and similar
	tagSetNSCPInlineFormat  = 14 // Netscape-specific inline formatting
	tagSetNSCPLayers        = 15 // Netscape layer tags
	tagSetNSCPForms         = 16 // Netscape form tags
	tagSetNSCPBlockFormat   = 17 // Netscape block-formatting tags
	tagSetNSCPServer        = 18 // the Netscape <SERVER> tag
	tagSetMSFTDocFormat     = 19 // Micro$oft-specific document formatting
	tagSetMSFTInlineFormat  = 20 // Micro$oft-specific inline formatting
	tagSetMSFTBlockFormat   = 21 // Micro$oft-specific block formatting
	tagSetMSFTActiveContent = 22 // Micro$oft-specific active content
	tagSetServerPage        = 23 // server-side page use
	tagSetJavaServer        = 24 // Java server page use
	tagSetComment           = 25 // HTML comments
)

// Functions used inside the tag to implement "overridden" behavior.
type causeLineBreakFunc func(*tag, bool) bool
type closingTagFunc func(*tag) string
type rewriteContentsFunc func(*tag, string, bool, htmlCheckerBackend) string

// tag is a structure describing a particular HTML tag.
type tag struct {
	name        string              // tag name, upper case
	index       int                 // index in the array
	lineBreak   bool                // does the tag cause line breaks?
	allowClose  bool                // is a close form of the tag allowed?
	balanceTags bool                // do we need to balance open and close tags?
	clb         causeLineBreakFunc  // does this tag cause line breaks?
	ct          closingTagFunc      // generate closing tag
	rwc         rewriteContentsFunc // rewrite the contents if necessary
}

// causeLineBreak returns true if the tag causes a line break.
func (t *tag) causeLineBreak(isClosing bool) bool {
	if t.clb == nil {
		return t.lineBreak
	}
	return t.clb(t, isClosing)
}

// makeClosingTag creates a closing tag for this one.
func (t *tag) makeClosingTag() string {
	if t.ct == nil {
		return ""
	}
	return t.ct(t)
}

// rewriteContents is a hook used to rewrite the contents of the tag.
func (t *tag) rewriteContents(contents string, isClosing bool, ctxt htmlCheckerBackend) string {
	if t.rwc == nil {
		return contents
	}
	return t.rwc(t, contents, isClosing, ctxt)
}

// createSimpleTag creates a structure for a simple tag.
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

// createWBRTag creates a structure for a WBR (word break) tag.
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

// stdClosingTag is the standard way a closing tag is made.
func stdClosingTag(tag *tag) string {
	return fmt.Sprintf("</%s>", tag.name)
}

// createOpenCloseTag creates a tag that has a specific open and close form.
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

// createListElementTag creates a tag that is part of a list.
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

// createBalancedTag creates a tag that should have opens and closes inherently balanced.
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

// createNOBRTag creates a NOBR (no break) tag.
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

// Patterns to be used in recognizing attributes in an <A> tag.
var hrefPattern = regexp.MustCompile(`(?i:href\s*=)`)
var targetPattern = regexp.MustCompile(`(?i:target\s*=)`)

// extractAttribute extracts an attribute value from the contents of an <A> tag.
func extractAttribute(s string) string {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return ""
	}
	if s[0] == '\'' || s[0] == '"' {
		p := strings.IndexByte(s[1:], s[0])
		if p < 0 {
			return ""
		}
		return s[1 : p+1]
	}
	return strings.Fields(s)[0]
}

// rewriteATagContents rewrites the contents of an <A> tag.
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

// createATag creates an <A> tag.
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

// tagNameToIndex is a mapping from tag names to indexes into the arrays.
var tagNameToIndex = make(map[string]int)

// tagIndexToObject contains the actual tags.
var tagIndexToObject = make([]*tag, 0, 50)

// tagIndexToSetId contains the set ID values for each tag.
var tagIndexToSetId = make([]int, 0, 50)

// tagMaxLength is the maximum length of a tag name.
var tagMaxLength = 0

// tagSetNameToSet is the listing of bit sets corresponding to the set names in configuration.
var tagSetNameToSet = make(map[string]*bitset.BitSet)

// enshrineTag adds a tag to our internal repository structures.
func enshrineTag(tag *tag, set int) {
	ndx := len(tagIndexToObject)
	tagIndexToObject = append(tagIndexToObject, tag)
	tagIndexToSetId = append(tagIndexToSetId, set)
	tag.index = ndx
	tagNameToIndex[tag.name] = ndx
	if len(tag.name) > tagMaxLength {
		tagMaxLength = len(tag.name)
	}
}

// init actually sets up the tag repository.
func init() {
	enshrineTag(createSimpleTag("!DOCTYPE", false), tagSetDocFormat)
	enshrineTag(createSimpleTag("%", false), tagSetServerPage)
	enshrineTag(createSimpleTag("%=", false), tagSetServerPage)
	enshrineTag(createSimpleTag("%@", false), tagSetServerPage)
	enshrineTag(createATag(), tagSetAnchor)
	enshrineTag(createBalancedTag("ABBR", false), tagSetInlineFormat)
	enshrineTag(createBalancedTag("ACRONYM", false), tagSetInlineFormat)
	enshrineTag(createBalancedTag("ADDRESS", true), tagSetBlockFormat)
	enshrineTag(createBalancedTag("APPLET", false), tagSetActiveContent)
	enshrineTag(createSimpleTag("AREA", false), tagSetImageMaps)
	enshrineTag(createBalancedTag("B", false), tagSetInlineFormat)
	enshrineTag(createSimpleTag("BASE", false), tagSetDocFormat)
	enshrineTag(createSimpleTag("BASEFONT", false), tagSetDocFormat)
	enshrineTag(createBalancedTag("BDO", false), tagSetInlineFormat)
	enshrineTag(createBalancedTag("BEAN", false), tagSetJavaServer)
	enshrineTag(createSimpleTag("BGSOUND", false), tagSetMSFTDocFormat)
	enshrineTag(createBalancedTag("BIG", false), tagSetInlineFormat)
	enshrineTag(createBalancedTag("BLINK", false), tagSetNSCPInlineFormat)
	enshrineTag(createBalancedTag("BLOCKQUOTE", true), tagSetBlockFormat)
	enshrineTag(createOpenCloseTag("BODY", false), tagSetDocFormat)
	enshrineTag(createSimpleTag("BR", true), tagSetBlockFormat)
	enshrineTag(createOpenCloseTag("BUTTON", false), tagSetForms)
	enshrineTag(createBalancedTag("CAPTION", true), tagSetTables)
	enshrineTag(createBalancedTag("CENTER", true), tagSetBlockFormat)
	enshrineTag(createBalancedTag("CITE", false), tagSetInlineFormat)
	enshrineTag(createBalancedTag("CODE", false), tagSetInlineFormat)
	enshrineTag(createSimpleTag("COL", true), tagSetTables)
	enshrineTag(createOpenCloseTag("COLGROUP", true), tagSetTables)
	enshrineTag(createBalancedTag("COMMENT", false), tagSetMSFTInlineFormat)
	enshrineTag(createListElementTag("DD"), tagSetBlockFormat)
	enshrineTag(createBalancedTag("DEL", false), tagSetChangeMarkup)
	enshrineTag(createBalancedTag("DFN", false), tagSetInlineFormat)
	enshrineTag(createBalancedTag("DIR", true), tagSetBlockFormat)
	enshrineTag(createBalancedTag("DIV", true), tagSetBlockFormat)
	enshrineTag(createBalancedTag("DL", true), tagSetBlockFormat)
	enshrineTag(createListElementTag("DT"), tagSetBlockFormat)
	enshrineTag(createBalancedTag("EM", false), tagSetInlineFormat)
	enshrineTag(createSimpleTag("EMBED", false), tagSetActiveContent)
	enshrineTag(createBalancedTag("FIELDSET", false), tagSetForms)
	enshrineTag(createBalancedTag("FONT", false), tagSetFontFormat)
	enshrineTag(createBalancedTag("FORM", false), tagSetForms)
	enshrineTag(createSimpleTag("FRAME", true), tagSetFrames)
	enshrineTag(createBalancedTag("FRAMESET", false), tagSetFrames)
	enshrineTag(createBalancedTag("H1", true), tagSetFontFormat)
	enshrineTag(createBalancedTag("H2", true), tagSetFontFormat)
	enshrineTag(createBalancedTag("H3", true), tagSetFontFormat)
	enshrineTag(createBalancedTag("H4", true), tagSetFontFormat)
	enshrineTag(createBalancedTag("H5", true), tagSetFontFormat)
	enshrineTag(createBalancedTag("H6", true), tagSetFontFormat)
	enshrineTag(createOpenCloseTag("HEAD", false), tagSetDocFormat)
	enshrineTag(createSimpleTag("HR", true), tagSetBlockFormat)
	enshrineTag(createOpenCloseTag("HTML", false), tagSetDocFormat)
	enshrineTag(createBalancedTag("I", false), tagSetInlineFormat)
	enshrineTag(createBalancedTag("IFRAME", true), tagSetFrames)
	enshrineTag(createBalancedTag("ILAYER", true), tagSetNSCPLayers)
	enshrineTag(createSimpleTag("IMG", false), tagSetImages)
	enshrineTag(createSimpleTag("INPUT", false), tagSetForms)
	enshrineTag(createBalancedTag("INS", false), tagSetChangeMarkup)
	enshrineTag(createSimpleTag("ISINDEX", false), tagSetForms)
	enshrineTag(createBalancedTag("KBD", false), tagSetInlineFormat)
	enshrineTag(createSimpleTag("KEYGEN", false), tagSetNSCPForms)
	enshrineTag(createBalancedTag("LABEL", false), tagSetForms)
	enshrineTag(createBalancedTag("LAYER", true), tagSetNSCPLayers)
	enshrineTag(createBalancedTag("LEGEND", false), tagSetForms)
	enshrineTag(createListElementTag("LI"), tagSetBlockFormat)
	enshrineTag(createSimpleTag("LINK", false), tagSetDocFormat)
	enshrineTag(createBalancedTag("LISTING", false), tagSetMSFTInlineFormat)
	enshrineTag(createBalancedTag("MAP", false), tagSetImageMaps)
	enshrineTag(createBalancedTag("MARQUEE", true), tagSetMSFTBlockFormat)
	enshrineTag(createBalancedTag("MENU", true), tagSetBlockFormat)
	enshrineTag(createSimpleTag("META", false), tagSetDocFormat)
	enshrineTag(createBalancedTag("MULTICOL", false), tagSetNSCPBlockFormat)
	enshrineTag(createNOBRTag(), tagSetBlockFormat)
	enshrineTag(createBalancedTag("NOEMBED", false), tagSetActiveContent)
	enshrineTag(createBalancedTag("NOFRAMES", false), tagSetFrames)
	enshrineTag(createBalancedTag("NOLAYER", false), tagSetNSCPLayers)
	enshrineTag(createBalancedTag("NOSCRIPT", false), tagSetActiveContent)
	enshrineTag(createBalancedTag("OBJECT", false), tagSetActiveContent)
	enshrineTag(createBalancedTag("OL", true), tagSetBlockFormat)
	enshrineTag(createBalancedTag("OPTGROUP", false), tagSetForms)
	enshrineTag(createListElementTag("OPTION"), tagSetForms)
	enshrineTag(createOpenCloseTag("P", true), tagSetBlockFormat)
	enshrineTag(createSimpleTag("PARAM", false), tagSetActiveContent)
	enshrineTag(createSimpleTag("PLAINTEXT", false), tagSetPreformat)
	enshrineTag(createBalancedTag("PRE", false), tagSetPreformat)
	enshrineTag(createBalancedTag("Q", false), tagSetInlineFormat)
	enshrineTag(createSimpleTag("RT", false), tagSetMSFTActiveContent)
	enshrineTag(createBalancedTag("RUBY", false), tagSetMSFTActiveContent)
	enshrineTag(createBalancedTag("S", false), tagSetInlineFormat)
	enshrineTag(createBalancedTag("SAMP", false), tagSetInlineFormat)
	enshrineTag(createBalancedTag("SCRIPT", false), tagSetActiveContent)
	enshrineTag(createBalancedTag("SELECT", false), tagSetForms)
	enshrineTag(createBalancedTag("SERVER", false), tagSetNSCPServer)
	enshrineTag(createBalancedTag("SERVLET", false), tagSetJavaServer)
	enshrineTag(createBalancedTag("SMALL", false), tagSetInlineFormat)
	enshrineTag(createSimpleTag("SPACER", false), tagSetNSCPInlineFormat)
	enshrineTag(createBalancedTag("SPAN", false), tagSetInlineFormat)
	enshrineTag(createBalancedTag("STRIKE", false), tagSetInlineFormat)
	enshrineTag(createBalancedTag("STRONG", false), tagSetInlineFormat)
	enshrineTag(createBalancedTag("STYLE", false), tagSetDocFormat)
	enshrineTag(createBalancedTag("SUB", false), tagSetInlineFormat)
	enshrineTag(createBalancedTag("SUP", false), tagSetInlineFormat)
	enshrineTag(createBalancedTag("TABLE", true), tagSetTables)
	enshrineTag(createOpenCloseTag("TBODY", true), tagSetTables)
	enshrineTag(createBalancedTag("TD", true), tagSetTables)
	enshrineTag(createBalancedTag("TEXTAREA", true), tagSetForms)
	enshrineTag(createOpenCloseTag("TFOOT", true), tagSetTables)
	enshrineTag(createBalancedTag("TH", true), tagSetTables)
	enshrineTag(createOpenCloseTag("THEAD", true), tagSetTables)
	enshrineTag(createBalancedTag("TITLE", false), tagSetDocFormat)
	enshrineTag(createBalancedTag("TR", true), tagSetTables)
	enshrineTag(createBalancedTag("TT", false), tagSetInlineFormat)
	enshrineTag(createBalancedTag("U", false), tagSetInlineFormat)
	enshrineTag(createBalancedTag("UL", true), tagSetBlockFormat)
	enshrineTag(createBalancedTag("VAR", false), tagSetInlineFormat)
	enshrineTag(createWBRTag(), tagSetBlockFormat)
	enshrineTag(createBalancedTag("XML", false), tagSetMSFTActiveContent)
	enshrineTag(createBalancedTag("XMP", false), tagSetNSCPInlineFormat)

	// Create the tag sets.
	bs := bitset.New(tagSetComment + 1)
	bs.Set(tagSetInlineFormat)
	bs.Set(tagSetAnchor)
	bs.Set(tagSetBlockFormat)
	bs.Set(tagSetFontFormat)
	bs.Set(tagSetImages)
	tagSetNameToSet["normal"] = bs
	bs = bitset.New(tagSetComment + 1)
	bs.Set(tagSetInlineFormat)
	tagSetNameToSet["restricted"] = bs
}

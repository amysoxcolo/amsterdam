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
	"errors"
	"fmt"
	"maps"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"git.erbosoft.com/amy/amsterdam/util"
	"github.com/bits-and-blooms/bitset"
	log "github.com/sirupsen/logrus"
)

// HTMLChecker is a component that checks HTML and reformats it as needed.
type HTMLChecker interface {
	Append(string) error
	Finish() error
	Reset()
	Value() (string, error)
	Length() (int, error)
	Lines() (int, error)
	Counter(string) (int, error)
	GetContext(string) any
	SetContext(string, any)
	ExternalRefs() ([]*url.URL, error)
	InternalRefs() ([]string, error)
}

var ErrAlreadyFinished = errors.New("the HTML checker has already finished")
var ErrNotYetFinished = errors.New("the HTML checker has not yet been finished")

type htmlCheckerBackend interface {
	getCheckerAttrValue(string) string
	sendTagMessage(string)
	getCheckerContextValue(string) any
	addExternalRef(*url.URL)
	addInternalRef(string)
}

// State constants for the state machine.
const (
	stateWhitespace = 0
	stateChars      = 1
	stateLeftAngle  = 2
	stateTag        = 3
	stateParen      = 4
	stateTagQuote   = 5
	stateNewline    = 6
)

// htmlMarginSlop is a number of characters at the end of the line used to control word-wrapping.
const htmlMarginSlop = 5

// hyphApos is used to find hyphens and apostrophes.
const hyphApos = "-'"

type htmlCheckerImpl struct {
	config             *HTMLCheckerConfig
	started            bool
	finished           bool
	state              int
	quoteChar          byte
	parenLevel         int
	columns            int
	lines              int
	noBreakCount       int
	triggerWBR         bool
	outputBuffer       strings.Builder
	tempBuffer         strings.Builder
	tagStack           *util.Stack[*tag]
	counters           map[string]*countingRewriter
	stringRewriters    []rewriter
	wordRewriters      []rewriter
	tagRewriters       []rewriter
	parenRewriters     []rewriter
	outputFilters      []outputFilter
	rawOutputFilters   []outputFilter
	contextData        map[string]any
	externalReferences map[*url.URL]bool
	internalReferences map[string]bool
	tagSet             *bitset.BitSet
}

func (ht *htmlCheckerImpl) copyRewriters(dest []rewriter, source []string) {
	for i := range source {
		rw, ok := rewriterRegistry[source[i]]
		if ok {
			if rw.Name() != "" {
				crw := MakeCountingRewriter(rw)
				ht.counters[rw.Name()] = crw
				rw = crw
			}
			dest[i] = rw
		} else {
			log.Errorf("rewriter %s is not found", source[i])
		}
	}
}

func (ht *htmlCheckerImpl) copyOutputFilters(dest []outputFilter, source []string) {
	for i := range source {
		f, ok := outputFilterRegistry[source[i]]
		if ok {
			dest[i] = f
		} else {
			log.Errorf("filter %s is not found", source[i])
		}
	}
}

func AmNewHTMLChecker(configName string) (HTMLChecker, error) {
	config, ok := configsRegistry[configName]
	if !ok {
		return nil, fmt.Errorf("configuration %s not found", configName)
	}
	tset, ok := tagSetNameToSet[config.TagSet]
	if !ok {
		return nil, fmt.Errorf("tag set %s not found", config.TagSet)
	}
	rc := htmlCheckerImpl{
		config:             config,
		started:            false,
		finished:           false,
		state:              stateWhitespace,
		parenLevel:         0,
		columns:            0,
		lines:              0,
		noBreakCount:       0,
		triggerWBR:         false,
		tagStack:           util.NewStack[*tag](),
		counters:           make(map[string]*countingRewriter),
		stringRewriters:    make([]rewriter, len(config.StringRewriters)),
		wordRewriters:      make([]rewriter, len(config.WordRewriters)),
		tagRewriters:       make([]rewriter, len(config.TagRewriters)),
		parenRewriters:     make([]rewriter, len(config.ParenRewriters)),
		outputFilters:      make([]outputFilter, len(config.OutputFilters)),
		rawOutputFilters:   make([]outputFilter, len(config.RawOutputFilters)),
		contextData:        make(map[string]any),
		externalReferences: make(map[*url.URL]bool),
		internalReferences: make(map[string]bool),
		tagSet:             tset,
	}
	rc.copyRewriters(rc.stringRewriters, config.StringRewriters)
	rc.copyRewriters(rc.wordRewriters, config.WordRewriters)
	rc.copyRewriters(rc.tagRewriters, config.TagRewriters)
	rc.copyRewriters(rc.parenRewriters, config.ParenRewriters)
	rc.copyOutputFilters(rc.outputFilters, config.OutputFilters)
	rc.copyOutputFilters(rc.rawOutputFilters, config.RawOutputFilters)
	return &rc, nil
}

func (ht *htmlCheckerImpl) getCheckerAttrValue(name string) string {
	if name == "ANCHORTAIL" {
		return ht.config.AnchorTail
	}
	return ""
}

func (ht *htmlCheckerImpl) sendTagMessage(msg string) {
	switch msg {
	case "NOBR":
		ht.noBreakCount++
	case "/NOBR":
		ht.noBreakCount--
	case "WBR":
		ht.triggerWBR = true
	}
}

func (ht *htmlCheckerImpl) getCheckerContextValue(name string) any {
	return ht.contextData[name]
}

func (ht *htmlCheckerImpl) addExternalRef(ref *url.URL) {
	ht.externalReferences[ref] = true
}

func (ht *htmlCheckerImpl) addInternalRef(ref string) {
	ht.internalReferences[ref] = true
}

func (ht *htmlCheckerImpl) rewriterAttrValue(name string) string {
	return ht.getCheckerAttrValue(name)
}

func (ht *htmlCheckerImpl) rewriterContextValue(name string) any {
	return ht.contextData[name]
}

func (ht *htmlCheckerImpl) emitRune(ch rune, filters []outputFilter, countCols bool) {
	handled := false
	if len(filters) > 0 {
		// try each output filter to see what we can do
		for _, of := range filters {
			handled = of.tryOutputRune(ht.outputBuffer, ch)
			if handled {
				break // found a filter to handle it, done
			}
		}
		if !handled { // output the raw character
			ht.outputBuffer.WriteRune(ch)
		}
		if countCols && ht.config.WordWrap > 0 {
			ht.columns++
		}
	}
}

func (ht *htmlCheckerImpl) emitString(str string, filters []outputFilter, countCols bool) {
	if str == "" {
		return
	}
	realCountCols := countCols && (ht.config.WordWrap > 0)
	if len(filters) == 0 {
		// if there are no filters, just output the whole thing
		ht.outputBuffer.WriteString(str)
		if realCountCols {
			ht.columns += utf8.RuneCountInString(str)
		}
		return
	}
	temp := str
	for len(temp) > 0 {
		// We output as much of the string as we possibly can at once. Assume, for now, we'll output the whole thing.
		outputLen := len(temp)

		// Now look at each of the output filters to see if we should try outputting a lesser amount
		// (i.e. does the string contain a "stopper" that one of the filters would like to mogrify?)
		var stopper outputFilter = nil
		for _, of := range filters {
			// find the length of characters that DOESN'T match this filter
			lnm := of.lengthNoMatch(temp)
			if lnm >= 0 && lnm < outputLen {
				// we've found a new stopper - record the length and the filter
				outputLen = lnm
				stopper = of
			}
			if outputLen <= 0 {
				break // nothing left to do here
			}
		}
		if outputLen > 0 {
			// move over the unaltered characters first
			ht.outputBuffer.WriteString(temp[:outputLen])
			if realCountCols {
				ht.columns += utf8.RuneCountInString(temp[:outputLen])
			}
		}
		if stopper != nil {
			// one of the output filters stopped us, try invoking it
			tmpch, bsiz := utf8.DecodeRuneInString(temp[outputLen:])
			outputLen += bsiz
			if !stopper.tryOutputRune(ht.outputBuffer, tmpch) {
				ht.outputBuffer.WriteRune(tmpch)
			}
			if realCountCols {
				ht.columns++
			}
		}
		// Chop the string and go around again.
		if outputLen == len(temp) {
			temp = ""
		} else if outputLen > 0 {
			temp = temp[outputLen:]
		}
	}
}

func (ht *htmlCheckerImpl) emitLineBreak() {
	ht.emitString("\r\n", ht.rawOutputFilters, false)
	if ht.config.WordWrap > 0 {
		ht.columns = 0
	}
	ht.lines++
}

func (ht *htmlCheckerImpl) emitPossibleLineBreak() {
	if ht.config.WordWrap > 0 && ht.noBreakCount <= 0 && ht.columns >= ht.config.WordWrap {
		ht.emitLineBreak()
	}
}

func (ht *htmlCheckerImpl) ensureSpaceOnLine(nchars int) {
	if ht.config.WordWrap > 0 && ht.noBreakCount <= 0 {
		// add a line break if needed here
		remainSpace := ht.config.WordWrap - ht.columns
		if remainSpace < nchars {
			ht.emitLineBreak()
		}
	}
}

func (ht *htmlCheckerImpl) emitMarkupData(md *markupData) {
	if !md.rescan {
		ht.ensureSpaceOnLine(len(md.text))
		ht.emitString(md.beginMarkup, ht.rawOutputFilters, false)
		ht.emitString(md.text, ht.outputFilters, true)
		ht.emitString(md.endMarkup, ht.rawOutputFilters, false)
	}
}

func (ht *htmlCheckerImpl) emitBracketedMarkupData(md *markupData, prefix rune, suffix rune) {
	if !md.rescan {
		l := len(md.text)
		if l > 0 {
			l += 2
		}
		ht.ensureSpaceOnLine(l)
		if len(md.text) > 0 {
			ht.emitRune(prefix, ht.outputFilters, true)
		}
		ht.emitString(md.beginMarkup, ht.rawOutputFilters, false)
		ht.emitString(md.text, ht.outputFilters, true)
		ht.emitString(md.endMarkup, ht.rawOutputFilters, false)
		if len(md.text) > 0 {
			ht.emitRune(suffix, ht.outputFilters, true)
		}
	}
}

func (ht *htmlCheckerImpl) doFlushWhitespace() {
	outputLen := ht.tempBuffer.Len()
	if outputLen > 0 {
		forceLineBreak := false
		if ht.config.WordWrap > 0 && ht.noBreakCount <= 0 {
			// adjust output if necessary for wordwrapping
			remainSpace := ht.config.WordWrap - ht.columns
			if remainSpace < outputLen {
				outputLen = remainSpace
			}
			if outputLen <= 0 {
				// this means that NONE of the whitespace would fit on this line...add a line break
				forceLineBreak = true
				outputLen = 0
			}
		}
		if forceLineBreak {
			ht.emitLineBreak()
		}
		if outputLen > 0 {
			ht.emitString(ht.tempBuffer.String()[:outputLen], ht.outputFilters, true)
		}
		ht.tempBuffer.Reset()
	}
}

func (ht *htmlCheckerImpl) doFlushNewlines() {
	// Measure the number of line breaks we have.
	lineBreaks, crs := 0, 0
	for ch := range []byte(ht.tempBuffer.String()) {
		switch ch {
		case '\r':
			crs++
		case '\n':
			crs = 0
			lineBreaks++
		}
	}
	if crs > 0 {
		lineBreaks++
	}

	// Adjust the number of line breaks if rewrap is in effect.
	if ht.config.Rewrap {
		if lineBreaks < 2 {
			// convert a single line break to whitespace
			ht.tempBuffer.Reset()
			ht.tempBuffer.WriteByte(' ')
			ht.state = stateWhitespace
			return
		} else {
			lineBreaks = 2 // compress out multiple blank lines
		}
	}

	for lineBreaks > 0 {
		ht.emitLineBreak()
		lineBreaks--
	}
	ht.tempBuffer.Reset()
	ht.state = stateWhitespace
}

func (ht *htmlCheckerImpl) emitFromStartOfTempBuffer(nrunes int) {
	if nrunes > 0 {
		if ht.config.WordWrap > 0 && ht.noBreakCount <= 0 {
			for nrunes > 0 {
				curlen := min(nrunes, ht.config.WordWrap-ht.columns)
				if curlen > 0 {
					s := ht.tempBuffer.String()
					bcurlen := util.RunesToBytes(s, curlen)
					ht.emitString(s[:bcurlen], ht.outputFilters, true)
					ht.tempBuffer.Reset()
					ht.tempBuffer.WriteString(s[bcurlen:])
					nrunes -= curlen
				}
				if ht.columns >= ht.config.WordWrap {
					ht.emitLineBreak()
				}
			}
		} else {
			s := ht.tempBuffer.String()
			bnrunes := util.RunesToBytes(s, nrunes)
			ht.emitString(s[:bnrunes], ht.outputFilters, true)
			ht.tempBuffer.Reset()
			ht.tempBuffer.WriteString(s[bnrunes:])
		}
	}
}

func (ht *htmlCheckerImpl) attemptRewrite(rewriters []rewriter, data string) *markupData {
	for _, r := range rewriters {
		rc := r.Rewrite(data, ht)
		if rc != nil {
			return rc
		}
	}
	return nil
}

func (ht *htmlCheckerImpl) doFlushString() bool {
	md := ht.attemptRewrite(ht.stringRewriters, ht.tempBuffer.String())
	if md != nil {
		ht.emitMarkupData(md)
		ht.tempBuffer.Reset()
		if md.rescan {
			ht.parse(md.all())
			return true
		}
		return false
	}

	first := true
	for ht.tempBuffer.Len() > 0 {
		sublen, isWord := util.WordRunLength(ht.tempBuffer.String())
		if isWord {
			// we want to check the word, but first we must eliminate leading hyphens and apostrophes
			hyphCount := 0
			for _, ch := range ht.tempBuffer.String() {
				if hyphCount == sublen || !strings.ContainsRune(hyphApos, ch) {
					break
				}
				hyphCount++
			}
			ht.emitFromStartOfTempBuffer(hyphCount)
			sublen -= hyphCount

			// now determine how many hyphens/apostrophes there are at the end of the word
			runeArray := []rune(ht.tempBuffer.String())
			wordLen := sublen
			hyphCount = 0
			for wordLen > 0 && strings.ContainsRune(hyphApos, runeArray[wordLen-1]) {
				hyphCount++
				wordLen--
			}

			if wordLen > 0 {
				// extract the word and remove it from the start of the buffer
				word := string(runeArray[:wordLen])
				lw := len(word)
				s := ht.tempBuffer.String()
				ht.tempBuffer.Reset()
				ht.tempBuffer.WriteString(s[lw:])

				// try to rewrite this word
				md := ht.attemptRewrite(ht.wordRewriters, word)
				if md != nil {
					// emit and/or reparse
					ht.emitMarkupData(md)
					if md.rescan {
						ht.parse(md.all())
					}
				} else {
					// just output the word normally
					ht.ensureSpaceOnLine(wordLen)
					ht.emitString(word, ht.outputFilters, true)
				}
			}

			// now emit the rest of the hyphens/apostrophes
			ht.emitFromStartOfTempBuffer(hyphCount)

		} else {
			// emit this many characters, line-breaking where required
			totalRunes := utf8.RuneCountInString(ht.tempBuffer.String())
			if sublen == totalRunes && !first && sublen <= htmlMarginSlop {
				// This is intended to handle a small run of non-word characters at the end of a string (i.e.
				// followed by whitespace) that should stay on the same line with its preceding word, to
				// eliminate "funnies" in punctuation formatting.
				ht.emitString(ht.tempBuffer.String(), ht.outputFilters, true)
				ht.tempBuffer.Reset()
				break
			}

			// This is kind of the inverse of the above check; if we have a small run of non-word
			// characters at the START of a word (preceded by whitespace and followed by at least
			// one word character), then ensure that we can keep that word and its prefixing non-word
			// characters on the same line (again, avoiding "funnies" in formatting).
			if sublen < totalRunes && first && sublen <= htmlMarginSlop {
				fwLen, _ := util.WordRunLengthAfterPrefix(ht.tempBuffer.String(), sublen)
				ht.ensureSpaceOnLine(sublen + fwLen)
			}
			ht.emitFromStartOfTempBuffer(sublen)
		}
		first = false
	}
	return false
}

func (ht *htmlCheckerImpl) handleAsHTML() bool {
	ht.triggerWBR = false
	tempString := ht.tempBuffer.String()
	// Figure out where the start of the command word is.
	startCmd := 0
	closingTag := false
	if startCmd < len(tempString) && tempString[startCmd] == '/' {
		startCmd++
		closingTag = true
	}

	// now figure out where it ends
	endCmd := startCmd
	for endCmd < len(tempString) {
		if unicode.IsSpace(rune(tempString[endCmd])) {
			break
		}
		endCmd++
	}

	if endCmd == startCmd || (endCmd-startCmd) > tagMaxLength {
		// command word is empty or is too long to be an HTML tag
		return false
	}
	possTagName := tempString[startCmd:endCmd]
	tagIndex, ok := tagNameToIndex[strings.ToUpper(possTagName)]
	if !ok {
		// not a known HTML tag
		return false
	}
	tag := tagIndexToObject[tagIndex]
	if closingTag && !tag.allowClose {
		// it's a closing tag and this tag doesn't permit the "close" form
		return false
	}
	tagSetID := tagIndexToSetId[tagIndex]
	if !ht.tagSet.Test(uint(tagSetID)) {
		// the tag is not allowed - discard it, if one of the flags is set in the config
		return ht.config.DiscardHTML || ht.config.DiscardRejected
	}
	if !ht.config.DiscardHTML && tag.balanceTags {
		// this tag needs to be balanced - here's where we manipulate the stack
		var valid bool
		if closingTag {
			valid = ht.tagStack.RemoveMostRecent(tag)
		} else {
			ht.tagStack.Push(tag)
			valid = true
		}
		if !valid {
			return false
		}
	}

	// Give the tag object one last chance to dictate what we do with the tag.
	realTagData := tag.rewriteContents(tempString, closingTag, ht)
	if realTagData == "" || ht.config.DiscardHTML {
		return true
	}

	// Emit the tag to the output.
	ht.emitRune('<', ht.rawOutputFilters, false)
	ht.emitString(realTagData, ht.rawOutputFilters, false)
	ht.emitRune('>', ht.rawOutputFilters, false)

	logicalLineBreak := false
	if ht.triggerWBR && !closingTag && ht.noBreakCount > 0 {
		// word break is logical line break, but only within no-break tags
		logicalLineBreak = true
	} else {
		logicalLineBreak = tag.causeLineBreak(closingTag)
	}
	if logicalLineBreak {
		ht.columns = 0
	}
	return true
}

func (ht *htmlCheckerImpl) containsHTMLComment() bool {
	return ht.tempBuffer.Len() >= 3 && strings.HasPrefix(ht.tempBuffer.String(), "!--")
}

func (ht *htmlCheckerImpl) containsCompleteHTMLComment() bool {
	if ht.tempBuffer.Len() >= 5 {
		s := ht.tempBuffer.String()
		return strings.HasPrefix(s, "!--") && strings.HasSuffix(s, "--")
	}
	return false
}

func (ht *htmlCheckerImpl) containsXMLConstruct() bool {
	tempString := ht.tempBuffer.String()
	ptr := 0
	if len(tempString) > 1 && tempString[0] == '/' {
		ptr++
	}
	for ptr < len(tempString) {
		if tempString[ptr] == ':' {
			return true
		} else if unicode.IsSpace(rune(tempString[ptr])) {
			break
		}
		ptr++
	}
	return false
}

func (ht *htmlCheckerImpl) finishTag() {
	if ht.containsHTMLComment() {
		if ht.containsCompleteHTMLComment() {
			if !ht.config.DiscardComments {
				// output the comment in the raw
				ht.emitRune('<', ht.rawOutputFilters, false)
				ht.emitString(ht.tempBuffer.String(), ht.rawOutputFilters, false)
				ht.emitRune('>', ht.rawOutputFilters, false)
				// clear state and retun to parsing
				ht.tempBuffer.Reset()
				ht.state = stateWhitespace
			}
		}
		return
	}
	if ht.handleAsHTML() {
		// this was valid HTML, we're done
		ht.tempBuffer.Reset()
		ht.state = stateWhitespace
		return
	}

	// try to handle it with a tag rewriter
	md := ht.attemptRewrite(ht.tagRewriters, ht.tempBuffer.String())
	if md != nil {
		ht.emitBracketedMarkupData(md, '<', '>')
		ht.tempBuffer.Reset()
		ht.state = stateWhitespace
		if md.rescan {
			ht.tempBuffer.WriteByte('<')
			ht.state = stateChars
			ht.parse(md.all() + ">")
		}
		return
	}

	if ht.config.DiscardXML && ht.containsXMLConstruct() {
		// this tag is an XML construct, and needs to be discarded
		ht.tempBuffer.Reset()
		ht.state = stateWhitespace
		return
	}

	// This tag has been rejected! process it normally as character data
	rejection := ht.tempBuffer.String()
	ht.tempBuffer.Reset()
	ht.tempBuffer.WriteByte('<')
	ht.state = stateChars
	if len(rejection) > 0 {
		ht.parse(rejection)
	}
	ht.parse(">")
}

func (ht *htmlCheckerImpl) finishParen() {
	// Try to handle the element using a paren rewriter
	md := ht.attemptRewrite(ht.parenRewriters, ht.tempBuffer.String())
	if md != nil {
		ht.emitBracketedMarkupData(md, '(', ')')
		ht.tempBuffer.Reset()
		ht.state = stateWhitespace
		ht.parenLevel = 0
		if md.rescan {
			ht.tempBuffer.WriteByte('(')
			ht.state = stateChars
			ht.parse(md.all() + ")")
		}
		return
	}

	// Tag rejected! Process it normally as character data.
	rejection := ht.tempBuffer.String()
	ht.tempBuffer.Reset()
	ht.tempBuffer.WriteByte('(')
	ht.state = stateChars
	ht.parenLevel = 0
	if len(rejection) > 0 {
		ht.parse(rejection)
	}
	ht.parse(")")
}

func (ht *htmlCheckerImpl) parse(str string) {
	i := 0
	for i < len(str) {
		ch := str[i]
		switch ht.state {
		case stateWhitespace:
			switch ch {
			case ' ', '\t': // append space and tab verbatim
				ht.tempBuffer.WriteByte(ch)
				i++
			case '\r', '\n': // flush and go to Newline state
				ht.doFlushWhitespace()
				ht.state = stateNewline
				ht.tempBuffer.WriteByte(ch)
				i++
			case '<':
				ht.doFlushWhitespace()
				if ht.config.Angles {
					ht.state = stateLeftAngle
				} else {
					// process < as ordinary character
					ht.state = stateChars
					ht.tempBuffer.WriteByte(ch)
				}
				i++
			case '(':
				ht.doFlushWhitespace()
				if ht.config.Parens {
					ht.state = stateParen
				} else {
					// process ( as ordinary character)
					ht.state = stateChars
					ht.tempBuffer.WriteByte(ch)
				}
				i++
			case '\\': // backslash processing is tricky - go to Chars state to handle it
				ht.doFlushWhitespace()
				ht.state = stateChars
			default:
				ht.doFlushWhitespace()
				ht.state = stateChars
				ht.tempBuffer.WriteByte(ch)
				i++
			}
		case stateChars:
			switch ch {
			case ' ', '\t': // go to Whitespace state
				ht.doFlushString()
				ht.state = stateWhitespace
				ht.tempBuffer.WriteByte(ch)
				i++
			case '\r', '\n': // go to Newline state
				ht.doFlushString()
				ht.state = stateNewline
				ht.tempBuffer.WriteByte(ch)
				i++
			case '<': // may be a start of tag
				if ht.config.Angles {
					ht.doFlushString()
					ht.state = stateLeftAngle
				} else {
					ht.tempBuffer.WriteByte(ch)
				}
				i++
			case '\\':
				if i < (len(str) - 1) {
					i++
					ch = str[i]
					if (ch == '(' && ht.config.Parens) || (ch == '<' && ht.config.Angles) {
						// append the escaped character, omitting the backslash
						ht.tempBuffer.WriteByte(ch)
						i++
					} else {
						// append the backslash and hit the new character
						ht.tempBuffer.WriteByte('\\')
					}
				} else {
					// just append the backslash notrmally
					ht.tempBuffer.WriteByte(ch)
					i++
				}
			default: // just append the next character
				ht.tempBuffer.WriteByte(ch)
				i++
			}
		case stateLeftAngle:
			switch ch {
			case ' ', '\t', '\r', '\n': // output <, go to Whitespace state
				ht.emitRune('<', ht.outputFilters, true)
				ht.state = stateWhitespace
			case '<': // output < and stay in this state
				ht.emitRune('<', ht.outputFilters, true)
				i++
			default:
				ht.state = stateTag
				ht.tempBuffer.WriteByte(ch)
				i++
			}
		case stateTag:
			switch ch {
			case '>': // finish the tag - this changes the state, and possibly calls parse() recursively
				ht.finishTag()
				i++
			case '\'', '"': // go into "quote string" state inside the tag
				ht.tempBuffer.WriteByte(ch)
				ht.state = stateTagQuote
				ht.quoteChar = ch
				i++
			default: // just append the character
				ht.tempBuffer.WriteByte(ch)
				i++
			}
		case stateParen:
			switch ch {
			case '(':
				ht.tempBuffer.WriteByte(ch)
				ht.parenLevel++
				i++
			case ')':
				if ht.parenLevel == 0 {
					ht.finishParen()
				} else {
					ht.tempBuffer.WriteByte(ch)
					ht.parenLevel--
				}
				i++
			default:
				ht.tempBuffer.WriteByte(ch)
				i++
			}
		case stateTagQuote:
			ht.tempBuffer.WriteByte(ch)
			if ch == ht.quoteChar {
				ht.state = stateTag
			}
			i++
		case stateNewline:
			if ch == '\r' || ch == '\n' {
				ht.tempBuffer.WriteByte(ch)
				i++
			} else {
				ht.doFlushNewlines()
			}
		}
	}
}

func (ht *htmlCheckerImpl) Append(str string) error {
	if ht.finished {
		return ErrAlreadyFinished
	}
	if !ht.started {
		ht.started = true
	}
	if str != "" {
		ht.parse(str)
	}
	return nil
}

func (ht *htmlCheckerImpl) Finish() error {
	if ht.finished {
		return ErrAlreadyFinished
	}
	if !ht.started {
		ht.started = true
	}
	// This is the "end parse" loop, in which we resolve any funny state the parser has
	// found itself in and clear out the internal buffers.
	running := true
	for running {
		running = false // make sure we stop unless this is set to true
		switch ht.state {
		case stateWhitespace, stateNewline:
			// do nothing - discard whitespace or newlines at end
		case stateChars:
			running = ht.doFlushString() // flush the temporary buffer
		case stateLeftAngle:
			// just emit a left angle character
			ht.emitPossibleLineBreak()
			ht.emitRune('<', ht.outputFilters, true)
		case stateTag, stateTagQuote:
			// we won't finish this tag, so it's automagically rejected
			rejection := ht.tempBuffer.String()
			ht.tempBuffer.Reset()
			ht.tempBuffer.WriteByte('<')
			ht.state = stateChars
			if len(rejection) > 0 {
				ht.parse(rejection)
			}
			running = true
		case stateParen:
			rejection := ht.tempBuffer.String()
			ht.tempBuffer.Reset()
			ht.tempBuffer.WriteByte('(')
			ht.state = stateChars
			ht.parenLevel = 0
			if len(rejection) > 0 {
				ht.parse(rejection)
			}
			running = true
		}
	}

	// Now close all the HTML tags that were left open.
	for !ht.tagStack.IsEmpty() {
		tag, _ := ht.tagStack.Pop()
		ht.outputBuffer.WriteString(tag.makeClosingTag())
	}

	ht.lines++
	ht.finished = true
	return nil
}

func (ht *htmlCheckerImpl) Reset() {
	ht.started = false
	ht.finished = false
	ht.triggerWBR = false
	ht.state = stateWhitespace
	ht.quoteChar = byte(0)
	ht.columns = 0
	ht.lines = 0
	ht.parenLevel = 0
	ht.outputBuffer.Reset()
	for u := range ht.externalReferences {
		delete(ht.externalReferences, u)
	}
	for k := range ht.internalReferences {
		delete(ht.internalReferences, k)
	}
	for c := range maps.Values(ht.counters) {
		c.Reset()
	}
}

func (ht *htmlCheckerImpl) Value() (string, error) {
	if ht.finished {
		return ht.outputBuffer.String(), nil
	}
	return "", ErrNotYetFinished
}

func (ht *htmlCheckerImpl) Length() (int, error) {
	if ht.finished {
		return ht.outputBuffer.Len(), nil
	}
	return 0, ErrNotYetFinished
}

func (ht *htmlCheckerImpl) Lines() (int, error) {
	if ht.finished {
		return ht.lines, nil
	}
	return 0, ErrNotYetFinished
}

func (ht *htmlCheckerImpl) Counter(name string) (int, error) {
	if ht.finished {
		cr, ok := ht.counters[name]
		if ok {
			return cr.GetCount(), nil
		}
		return 0, nil
	}
	return 0, ErrNotYetFinished
}

func (ht *htmlCheckerImpl) GetContext(name string) any {
	return ht.contextData[name]
}

func (ht *htmlCheckerImpl) SetContext(name string, value any) {
	ht.contextData[name] = value
}

func (ht *htmlCheckerImpl) ExternalRefs() ([]*url.URL, error) {
	if ht.finished {
		rc := make([]*url.URL, len(ht.externalReferences))
		p := 0
		for url := range maps.Keys(ht.externalReferences) {
			rc[p] = url
			p++
		}
		return rc, nil
	}
	return nil, ErrNotYetFinished
}

func (ht *htmlCheckerImpl) InternalRefs() ([]string, error) {
	if ht.finished {
		rc := make([]string, len(ht.internalReferences))
		p := 0
		for s := range maps.Keys(ht.internalReferences) {
			rc[p] = s
			p++
		}
	}
	return nil, ErrNotYetFinished
}

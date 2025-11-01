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
	"net/url"
	"strings"

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

var AlreadyFinished = errors.New("the HTML checker has already finished")
var NotYetFinished = errors.New("the HTML checker has not yet been finished")

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
		contextData:        make(map[string]any),
		externalReferences: make(map[*url.URL]bool),
		internalReferences: make(map[string]bool),
		tagSet:             tset,
	}
	rc.copyRewriters(rc.stringRewriters, config.StringRewriters)
	rc.copyRewriters(rc.wordRewriters, config.WordRewriters)
	rc.copyRewriters(rc.tagRewriters, config.TagRewriters)
	rc.copyRewriters(rc.parenRewriters, config.ParenRewriters)
	for i := range config.OutputFilters {
		f, ok := outputFilterRegistry[config.OutputFilters[i]]
		if ok {
			rc.outputFilters[i] = f
		} else {
			log.Errorf("filter %s is not found", config.OutputFilters[i])
		}
	}
	return &rc
}

func (ht *htmlCheckerImpl) emitString(str string, filters []outputFilter, countCols bool) {
	if str == "" {
		return
	}
	realCountCols := countCols && (ht.config.WordWrap > 0)
	if len(filters) == 0 {
		ht.outputBuffer.WriteString(str)
		if realCountCols {
			ht.columns += len(str)
		}
		return
	}
	temp := str
	for len(temp) > 0 {
		outputLen := len(temp)
		var stopper outputFilter = nil
		for _, of := range filters {
			lnm := of.lengthNoMatch(temp)
			if lnm >= 0 && lnm < outputLen {
				outputLen = lnm
				stopper = of
			}
			if outputLen <= 0 {
				break
			}
		}
		if outputLen > 0 {
			ht.outputBuffer.WriteString(temp[:outputLen])
			if realCountCols {
				ht.columns += outputLen
			}
		}
		if stopper != nil {
			tmpch := temp[outputLen]
			outputLen++
			if !stopper.tryOutputCharacter(ht.outputBuffer, tmpch) {
				ht.outputBuffer.WriteByte(tmpch)
			}
			if realCountCols {
				ht.columns++
			}
		}
		if outputLen == len(temp) {
			temp = ""
		} else if outputLen > 0 {
			temp = temp[outputLen:]
		}
	}
}

func (ht *htmlCheckerImpl) emitLineBreak() {

}

func (ht *htmlCheckerImpl) emitPossibleLineBreak() {
	if ht.config.WordWrap > 0 && ht.noBreakCount <= 0 && ht.columns >= ht.config.WordWrap {
		ht.emitLineBreak()
	}
}

func (ht *htmlCheckerImpl) doFlushString() bool {
	return false // TODO
}

func (ht *htmlCheckerImpl) parse(str string) {

}

func (ht *htmlCheckerImpl) Append(str string) error {
	if ht.finished {
		return AlreadyFinished
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
		return AlreadyFinished
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

		}
	}
}

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
	_ "embed"
	"os"

	"git.erbosoft.com/amy/amsterdam/config"
	log "github.com/sirupsen/logrus"
)

// SpellingDictionary is a simple dictionary interface.
type SpellingDictionary interface {
	Ready() bool
	Size() int
	CheckWord(string) bool
}

// ModSpellingDictionary is an intrerface to a modifiable spelling dictionary.
type ModSpellingDictionary interface {
	SpellingDictionary
	AddWord(string)
	DelWord(string)
	Clear()
}

//go:embed en-us.dict
var mainDict []byte

//go:embed supplement.dict
var supplementaryDict []byte

// SetupDicts sets up the dictionaries and the spelling rewriter.
func SetupDicts() {
	dicts := make([]SpellingDictionary, 2, 3)
	dicts[0] = LoadTrieDict(mainDict)
	dicts[1] = LoadTrieDict(supplementaryDict)
	if config.GlobalConfig.Posting.ExternalDictionary != "" {
		data, err := os.ReadFile(config.GlobalConfig.Posting.ExternalDictionary)
		if err == nil {
			ndict := LoadTrieDict(data)
			dicts = append(dicts, ndict)
		} else {
			log.Errorf("failed to load external dictionary %s: %v", config.GlobalConfig.Posting.ExternalDictionary, err)
		}
	}
	rw := spellingRewriter{
		dict: NewCompositeDict(dicts),
	}
	rewriterRegistry[rw.Name()] = &rw
}

// spellingRewriter is a rewriter that flags spelling errors.
type spellingRewriter struct {
	dict SpellingDictionary
}

// defaultBeginError is the markup that indicates the start of an error.
const defaultBeginError = "<span class=\"text-red-600 font-bold\">"

// defaultEndError is the markup that indicates the end of an error.
const defaultEndError = "</span>"

// Name returns the rewriter's name.
func (rw *spellingRewriter) Name() string {
	return "spelling"
}

/* Rewrite rewrites the given string data and adds markup before and after if needed.
 * Parameters:
 *     data - The data to be rewritten.
 *     svc - Services interface we can use.
 * Returns:
 *     Pointer to markup data, or nil.
 */
func (rw *spellingRewriter) Rewrite(data string, svc rewriterServices) *markupData {
	if rw.dict.CheckWord(data) {
		return nil
	}
	return &markupData{
		beginMarkup: defaultBeginError,
		text:        data,
		endMarkup:   defaultEndError,
		rescan:      false,
	}
}

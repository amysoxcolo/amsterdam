/*
 * Amsterdam Web Communities System
 * Copyright (c) 2025 Erbosoft Metaverse Design Solutions, All Rights Reserved
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * SPDX-License-Identifier: MPL-2.0
 */

// Package util contains utility definitions.
package util

import (
	_ "embed"
	"slices"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
)

//go:embed languages.txt
var knownLanguages string

// Language is a type for a list of all supported languages.
type Language struct {
	Tag  string // the BCP 47 tag, such as "en-US"
	Name string // the human-readable name, like "American English"
}

// cachedLanguageList is the cached language list.
var cachedLanguageList []Language = nil

// mapping from language tag names to actual language entries
var languageTagMapper map[string]*Language

// languageListMutex controls access to internalGetLanguageList.
var languageListMutex sync.Mutex

// AmLanguageList returns a list of all known languages.
func AmLanguageList() []Language {
	languageListMutex.Lock()
	defer languageListMutex.Unlock()
	if cachedLanguageList == nil {
		langs := strings.Split(knownLanguages, "\n")
		enNamer := display.English.Tags()
		cachedLanguageList = make([]Language, 0, len(langs))
		for _, l := range langs {
			tag, err := language.Parse(l)
			if err == nil {
				cachedLanguageList = append(cachedLanguageList, Language{
					Tag:  tag.String(),
					Name: enNamer.Name(tag),
				})
			} else {
				log.Errorf("*** PUKE on parsing language tag %s: %v", l, err)
			}
		}

		slices.SortFunc(cachedLanguageList, func(a Language, b Language) int {
			return strings.Compare(a.Name, b.Name)
		})
		languageTagMapper = make(map[string]*Language)
		for i := range cachedLanguageList {
			languageTagMapper[strings.ToLower(cachedLanguageList[i].Tag)] = &(cachedLanguageList[i])
		}
	}
	return cachedLanguageList
}

/* AmLanguageInLanguage displays a language name in any other language.
 * Parameters:
 *     lang - The language to be displayed.
 *     inLang - The language to display the other language's name in.
 * Returns:
 *     The full translated language name.
 */
func AmLanguageInLanguage(lang language.Tag, inLang language.Tag) string {
	namer := display.Tags(inLang)
	if namer == nil {
		namer = display.English.Tags()
	}
	s := namer.Name(lang)
	if s == "" {
		s = display.English.Tags().Name(lang)
	}
	return s
}

func init() {
	go AmLanguageList() // preload the list in the background
}

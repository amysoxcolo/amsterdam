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

// CompositeDictionary is a dictionary that wraps several base dictionaries, and adds some extra behavior.
type CompositeDictionary struct {
	dicts []SpellingDictionary
}

// Ready returns true if the dictionary has been fully loaded.
func (d *CompositeDictionary) Ready() bool {
	for _, sd := range d.dicts {
		if !sd.Ready() {
			return false
		}
	}
	return true
}

// Size returns the number of words in the dictionary.
func (d *CompositeDictionary) Size() int {
	rc := 0
	for _, sd := range d.dicts {
		rc += sd.Size()
	}
	return rc
}

// checkSimple passes a word to the subdictionaries to check it.
func (d *CompositeDictionary) checkSimple(word string) bool {
	for _, sd := range d.dicts {
		if sd.CheckWord(word) {
			return true
		}
	}
	return false
}

// checkHyphenates breaks a hyphenatewd work up into parts and checks each one.
func (d *CompositeDictionary) checkHyphenates(word string) bool {
	parts := strings.Split(word, "-")
	if len(parts) == 1 {
		return false // no hyphens
	}
	for _, frag := range parts {
		// each fragment greater than 1 character must be in dictionary
		if len(frag) > 1 {
			if !d.checkSimple(frag) {
				return false
			}
		}
	}
	return true
}

// CheckWord returns true if a word appears in the dictionary.
func (d *CompositeDictionary) CheckWord(word string) bool {
	if len(word) <= 1 {
		return true // words of length 1 get a free pass
	}
	realWord := strings.ToLower(word)
	if d.checkSimple(realWord) {
		return true
	}
	if strings.HasSuffix(realWord, "'s") {
		l := len(realWord)
		base := realWord[:l-2]
		if d.checkSimple(base) {
			return true
		}
		return d.checkHyphenates(base)
	}
	return d.checkHyphenates(realWord)
}

// NewCompositeDict wraps an array of SpellingDictionary objects up in a composite.
func NewCompositeDict(dicts []SpellingDictionary) *CompositeDictionary {
	return &CompositeDictionary{
		dicts: dicts,
	}
}

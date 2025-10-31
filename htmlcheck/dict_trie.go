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
	"bufio"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/derekparker/trie"
	log "github.com/sirupsen/logrus"
)

// TrieDictionary is a ModSpellingDictionary implemented using a trie.
type TrieDictionary struct {
	mutex  sync.Mutex
	loaded atomic.Bool
	trie   *trie.Trie
	count  int
}

// Ready lets us know if the dictionary is fully loaded.
func (d *TrieDictionary) Ready() bool {
	return d.loaded.Load()
}

// Size returns the number of words in the dictionary.
func (d *TrieDictionary) Size() int {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	return d.count
}

// CheckWord returns true if a word is in the dictionary, false if not.
func (d *TrieDictionary) CheckWord(word string) bool {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	_, rc := d.trie.Find(strings.ToLower(word))
	return rc
}

// AddWord adds a new word to the dictionary.
func (d *TrieDictionary) AddWord(word string) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.trie.Add(strings.ToLower(word), true)
	d.count++
}

// DelWord deletes a word from the dictionary.
func (d *TrieDictionary) DelWord(word string) {
	// not implemented for this type
}

// Clear removes all words from the dictionary.
func (d *TrieDictionary) Clear() {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.trie = trie.New()
	d.count = 0
}

// loadDict is a goroutine that loads the dictionary in the background.
func loadDict(d *TrieDictionary, words []byte) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	scanner := bufio.NewScanner(strings.NewReader(string(words)))
	for scanner.Scan() {
		word := strings.TrimSpace(scanner.Text())
		if word != "" {
			d.trie.Add(strings.ToLower(word), true)
		}
	}
	if err := scanner.Err(); err != nil {
		log.Fatalf("failed to load dictionary: %v", err)
	}
	d.loaded.Store(true)
}

// LoadTrieDict creates a TrieDictionary from a byte array that represents a word list (one word per line).
func LoadTrieDict(words []byte) *TrieDictionary {
	rc := TrieDictionary{
		loaded: atomic.Bool{},
		trie:   trie.New(),
		count:  0,
	}
	rc.loaded.Store(false)
	go loadDict(&rc, words)
	return &rc
}

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
	"strings"

	"github.com/bits-and-blooms/bitset"
)

// optionAlphabet is the alphabet from which OptionSets serialize to and from strings.
const optionAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!#$&()*+,-./:;<=>?@[]^_`{|}~"

// OptionSet is a bit set that can be persisted as a specially-constructed string.
type OptionSet struct {
	bits *bitset.BitSet
}

// Get retrieves the value of a bit from the given set.
func (s *OptionSet) Get(ndx uint) bool {
	return s.bits.Test(ndx)
}

// Set sets the value of a bit in the given set.
func (s *OptionSet) Set(ndx uint, v bool) {
	if v {
		s.bits = s.bits.Set(ndx)
	} else {
		s.bits = s.bits.Clear(ndx)
	}
}

// AsString returns the option set's value as a string.
func (s *OptionSet) AsString() string {
	var b strings.Builder
	for i, e := s.bits.NextSet(0); e; i, e = s.bits.NextSet(i + 1) {
		b.WriteByte(optionAlphabet[int(i)])
	}
	return b.String()
}

// Clone creates a clone of this OptionSet.
func (s *OptionSet) Clone() *OptionSet {
	return &OptionSet{bits: s.bits.Clone()}
}

// NewOptionSet creates and returns an empty option set.
func NewOptionSet() *OptionSet {
	return &OptionSet{bits: bitset.New(uint(len(optionAlphabet)))}
}

// OptionSetFromString converts a string into a corresponding OptionSet.
func OptionSetFromString(s string) *OptionSet {
	bs := bitset.New(uint(len(optionAlphabet)))
	for _, ch := range s {
		bs = bs.Set(uint(strings.IndexRune(optionAlphabet, ch)))
	}
	return &OptionSet{bits: bs}
}

// OptionCharFromIndex converts an integer into the matching character from the option alphabet.
func OptionCharFromIndex(ndx uint) string {
	if ndx > uint(len(optionAlphabet)) {
		return ""
	}
	return optionAlphabet[ndx : ndx+1]
}

/*
 * Amsterdam Web Communities System
 * Copyright (c) 2025 Erbosoft Metaverse Design Solutions, All Rights Reserved
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

// Package util contains utility definitions.
package util

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var numeric *regexp.Regexp = regexp.MustCompile(`^[0-9]+$`)

/* CapitalizeString changes the first character of the string to a capital.
 * Parameters:
 *     s - The string to be capitalized.
 * Returns:
 *     The capitalized string.
 */
func CapitalizeString(s string) string {
	runes := []rune(s)
	if len(runes) > 0 {
		runes[0] = unicode.ToUpper(runes[0])
		return string(runes)
	}
	return ""
}

/* SqlEscape escapes a string in SQL terms.
 * Parameters:
 *     s - The string to be escaped.
 *     wildcards - If true, also escape the wildcard characters % and _.
 * Returns:
 *     The escaped string.
 */
func SqlEscape(s string, wildcards bool) string {
	var sb strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '\\', 0, '\n', '\r', '\'', '"':
			sb.WriteByte('\\')
			sb.WriteByte(c)
		case '\032':
			sb.WriteByte('\\')
			sb.WriteByte('Z')
		case '%', '_':
			if wildcards {
				sb.WriteByte('\\')
			}
			sb.WriteByte(c)
		default:
			sb.WriteByte(c)
		}
	}
	return sb.String()
}

/* IsNumeric returns true if the string is numeric (all digits).
 * Parameters:
 *     s - String to be tested.
 * Returns:
 *     true if string is numeric, false if not.
 */
func IsNumeric(s string) bool {
	return numeric.MatchString(s)
}

/* RunesToBytes returns the number of bytes in a string counting the number of runes from the beginning.
 * Parameters:
 *     s - The string to work with.
 *     runeCount - The number of runes to count from the start of the string.
 * Returns:
 *     The corresponding number of bytes.
 */
func RunesToBytes(s string, runeCount int) int {
	bp := 0
	for runeCount > 0 {
		if bp >= len(s) {
			return len(s)
		}
		_, c := utf8.DecodeRuneInString(s[bp:])
		bp += c
		runeCount--
	}
	return bp
}

// IsRuneWord returns true if the given rune is part of a word.
func IsRuneWord(ch rune) bool {
	return unicode.IsLetter(ch) || ch == '-' || ch == '\''
}

/* WordRunLength calculates the number of runes at the start of the string that are either word or non-word characters.
 * Parameters:
 *     s - The string under test.
 * Returns:
 *     The run length in runes.
 *     true if the run is a length of word characters, false if it's a run of non-word characters.
 */
func WordRunLength(s string) (int, bool) {
	c1, initLen := utf8.DecodeRuneInString(s)
	wordChar := IsRuneWord(c1)
	rlen := 1
	for _, mch := range s[initLen:] {
		if IsRuneWord(mch) != wordChar {
			break
		}
		rlen++
	}
	return rlen, wordChar
}

/* WordRunLengthAfterPrefix calculates the number of runes after a certain number in the string
 * that are either word or non-word characters.
 * Parameters:
 *     s - The string under test.
 *     nrunes - The number of runes to skip at the start of the string.
 * Returns:
 *     The run length in runes.
 *     true if the run is a length of word characters, false if it's a run of non-word characters.
 */
func WordRunLengthAfterPrefix(s string, nrunes int) (int, bool) {
	ofs := 0
	for _, ch := range s {
		if nrunes == 0 {
			break
		}
		ofs += utf8.RuneLen(ch)
		nrunes--
	}
	return WordRunLength(s[ofs:])
}

/* Map applies a transformation function on all elements of an array of one type, turning it into an
 * array of another type.
 * Parameters:
 *     in - The input array to be transformed.
 *     fn - The function to be executed on each element.
 * Returns:
 *     The array of new elements.
 */
func Map[A, B any](in []A, fn func(A) B) []B {
	rc := make([]B, len(in))
	for i, v := range in {
		rc[i] = fn(v)
	}
	return rc
}

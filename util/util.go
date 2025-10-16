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
	"unicode"
)

var numeric *regexp.Regexp

func init() {
	re, err := regexp.Compile("^[0-9]+$")
	if err != nil {
		panic(err)
	}
	numeric = re
}

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

/* IsNumeric returns true if the string is numeric (all digits).
 * Parameters:
 *     s - String to be tested.
 * Returns:
 *     true if string is numeric, false if not.
 */
func IsNumeric(s string) bool {
	return numeric.MatchString(s)
}

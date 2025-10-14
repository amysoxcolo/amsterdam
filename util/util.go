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

import "unicode"

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

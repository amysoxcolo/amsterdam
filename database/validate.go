/*
 * Amsterdam Web Communities System
 * Copyright (c) 2025 Erbosoft Metaverse Design Solutions, All Rights Reserved
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */
// The database package contains database management and storage logic.
package database

import "strings"

const AMS_ID_CHARS = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-_~*'$"

// AmIsValidAmsterdamID returns true if the given string is a valid Amsterdam ID.
func AmIsValidAmsterdamID(test string) bool {
	if len(test) < 1 {
		return false
	}
	for _, r := range test {
		if !strings.ContainsRune(AMS_ID_CHARS, r) {
			return false
		}
	}
	return true
}

// AmIsValidAmsterdamIDChar returns true if the character is a valid character in an Amsterdam ID.
func AmIsValidAmsterdamIDChar(ch byte) bool {
	return strings.ContainsRune(AMS_ID_CHARS, rune(ch))
}

// AmIsValidPostLinkChar returns true if the character is a valid character in a post link.
func AmIsValidPostLinkChar(ch byte) bool {
	if strings.ContainsRune(AMS_ID_CHARS, rune(ch)) {
		return true
	}
	return ch == '.' || ch == '!'
}

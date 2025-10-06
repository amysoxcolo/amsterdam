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
	crand "crypto/rand"
	"io"
	mrand "math/rand"
)

// authAlphabet is the set of characters from which we generate auth strings.
const authAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz./"

// authStringLen is the standard lengtth of authentication strings.
const authStringLen = 32

// GenerateRandomAuthString generates a random authentication string.
func GenerateRandomAuthString() string {
	b := make([]byte, authStringLen)
	if _, err := io.ReadFull(crand.Reader, b); err != nil {
		// can't happen (at least on a modern OS)
		panic("failed to read random: " + err.Error())
	}
	for i := 0; i < authStringLen; i++ {
		b[i] = authAlphabet[int(b[i])%len(authAlphabet)]
	}
	return string(b)
}

// GenerateRandomConfirmationNumber generates a random 7-digit confirmation number.
func GenerateRandomConfirmationNumber() int32 {
	return mrand.Int31n(9000000) + 1000000
}

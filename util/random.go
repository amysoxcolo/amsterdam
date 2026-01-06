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
	"strings"

	log "github.com/sirupsen/logrus"
)

// authAlphabet is the set of characters from which we generate auth strings.
const authAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz./"

// authStringLen is the standard lengtth of authentication strings.
const authStringLen = 32

// syllabary is used to generate random passwords.
var syllabary = [...]string{
	"ba", "be", "bi", "bo", "bu",
	"da", "de", "di", "do", "du",
	"cha", "chi", "cho", "chu",
	"fa", "fe", "fi", "fo", "fu",
	"ga", "ge", "gi", "go", "gu",
	"ha", "he", "hi", "ho", "hu",
	"ja", "je", "ji", "jo", "ju",
	"ka", "ke", "ki", "ko", "ku",
	"la", "le", "li", "lo", "lu",
	"ma", "me", "mi", "mo", "mu",
	"na", "ne", "ni", "no", "nu",
	"pa", "pe", "pi", "po", "pu",
	"ra", "re", "ri", "ro", "ru",
	"sa", "se", "si", "so", "su",
	"sha", "she", "sho", "shu",
	"ta", "te", "ti", "to", "tu",
	"va", "ve", "vi", "vo", "vu",
	"wa", "we", "wi", "wo", "wu",
	"ya", "ye", "yi", "yo", "yu",
	"za", "ze", "zi", "zo", "zu",
}

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
	rc := mrand.Int31n(9000000) + 1000000
	for rc < 1000000 || rc > 9999999 {
		log.Errorf("*** GRCN out of range error! %d", rc)
		rc = mrand.Int31n(9000000) + 1000000
	}
	return rc
}

// GenerateRandomPassword generates a random password string.
func GenerateRandomPassword() string {
	var b strings.Builder
	rd := make([]byte, 7)
	if _, err := io.ReadFull(crand.Reader, rd); err != nil {
		// can't happen (at least on a modern OS)
		panic("failed to read random: " + err.Error())
	}
	for i := 0; i < 4; i++ { // add random syllables
		b.WriteString(syllabary[int(rd[i])%len(syllabary)])
	}
	for i := 4; i < 7; i++ { // add random digits
		b.WriteByte(byte('0' + int(rd[i])%10))
	}
	return b.String()
}

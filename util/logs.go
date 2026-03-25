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
	"time"

	log "github.com/sirupsen/logrus"
)

// MeasureTime is called via a defer, and prints the amount of time for a function.
func MeasureTime(funcName string) func() {
	start := time.Now()
	return func() {
		log.Debugf("-- Time for function \"%s\" is %v", funcName, time.Since(start))
	}
}

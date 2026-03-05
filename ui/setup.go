/*
 * Amsterdam Web Communities System
 * Copyright (c) 2025-2026 Erbosoft Metaverse Design Solutions, All Rights Reserved
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

// Package ui holds the support for the Amsterdam user interface, wrapping Echo and Jet templates.
package ui

import "slices"

// SetupUILayer sets up the UI layer, and returns a function to close it down again.
func SetupUILayer() func() {
	exitfuncs := make([]func(), 0, 2)
	setupTemplates()
	setupDialogs()
	setupMenuCache()
	setupResources()
	exitfuncs = append(exitfuncs, setupSessionManager())
	exitfuncs = append(exitfuncs, setupContext())
	return func() {
		slices.Reverse(exitfuncs)
		for _, f := range exitfuncs {
			f()
		}
	}
}

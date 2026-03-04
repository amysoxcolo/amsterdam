//go:build darwin

/*
 * Amsterdam Web Communities System
 * Copyright (c) 2025-2026 Erbosoft Metaverse Design Solutions, All Rights Reserved
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

// Package config contains support for Amsterdam site-wide configuration data.
package config

// configFileDirs returns the directories where an Amsterdam config file may be located.
func configFileDirs() []string {
	// this variant is for Apple macOS
	rc := make([]string, 0, 2)
	rc = append(rc, "/usr/local/etc/amsterdam", "/Library/Application Support/Amsterdam")
	return rc
}

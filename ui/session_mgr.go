/*
 * Amsterdam Web Communities System
 * Copyright (c) 2025 Erbosoft Metaverse Design Solutions, All Rights Reserved
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

// Package ui holds the support for the Amsterdam user interface, wrapping Echo and Jet templates.
package ui

import (
	"git.erbosoft.com/amy/amsterdam/config"
	"git.erbosoft.com/amy/amsterdam/database"
	"github.com/gorilla/sessions"
	"github.com/quasoft/memstore"
	log "github.com/sirupsen/logrus"
)

// SessionStore is the Gorilla session store used by Amsterdam.
var SessionStore sessions.Store

// SetupSessionManager sets up the session manager.
func SetupSessionManager() {
	SessionStore = memstore.NewMemStore([]byte(config.GlobalConfig.Rendering.CookieKey))
}

// setupAmSession sets up a newly created Amsterdam session.
func setupAmSession(session *sessions.Session) {
	u, err := database.AmGetAnonUser()
	if err == nil {
		session.Values["user_id"] = u.Uid
	} else {
		log.Errorf("Unable to load anon user: %v", err)
	}
}

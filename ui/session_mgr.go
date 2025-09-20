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
	log "github.com/sirupsen/logrus"
)

// SessionStore is the Gorilla session store used by Amsterdam.
var SessionStore sessions.Store

// SetupSessionManager sets up the session manager.
func SetupSessionManager() {
	log.Infof("Cookie key is %s", config.GlobalConfig.Rendering.CookieKey)
	SessionStore = sessions.NewCookieStore([]byte(config.GlobalConfig.Rendering.CookieKey))
}

// SetupAmSession sets up a newly created Amsterdam session.
func SetupAmSession(session *sessions.Session) {
	session.Values["temp"] = "Active"
	u, err := database.AmGetAnonUser()
	if err == nil {
		session.Values["user"] = u
	} else {
		log.Errorf("Unable to load anon user: %v", err)
	}
}

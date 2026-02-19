/*
 * Amsterdam Web Communities System
 * Copyright (c) 2025-2026 Erbosoft Metaverse Design Solutions, All Rights Reserved
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */
// The database package contains database management and storage logic.
package database

import (
	"git.erbosoft.com/amy/amsterdam/config"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

// amdb is the reference to the Amsterdam database.  Returns a function to close it down.
var amdb *sqlx.DB

// SetupDb sets up the database and associated items.
func SetupDb() (func(), error) {
	var fn1 func() = nil
	var fn2 func() = nil
	db, err := sqlx.Open(config.GlobalConfig.Database.Driver, config.GlobalConfig.Database.Dsn)
	if err == nil {
		amdb = db
		fn1 = setupAuditWriter()
		fn2 = setupIPBanSweep()
	}
	return func() {
		if fn2 != nil {
			fn2()
		}
		if fn1 != nil {
			fn1()
		}
		amdb.Close()
	}, err
}

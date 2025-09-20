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

import (
	"git.erbosoft.com/amy/amsterdam/config"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

// amdb is the reference to the Amsterdam database.
var amdb *sqlx.DB

// SetupDb sets up the database and associated items.
func SetupDb() error {
	db, err := sqlx.Open(config.GlobalConfig.Database.Driver, config.GlobalConfig.Database.Dsn)
	if err == nil {
		amdb = db
		// TODO: additional initialization
	}
	return err
}

// ClosedownDb closes down the database and associated items.
func ClosedownDb() {
	amdb.Close()
}

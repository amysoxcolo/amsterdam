/*
 * Amsterdam Web Communities System
 * Copyright (c) 2025-2026 Erbosoft Metaverse Design Solutions, All Rights Reserved
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * SPDX-License-Identifier: MPL-2.0
 */
// The database package contains database management and storage logic.
package database

import (
	"context"
	"fmt"
	"slices"

	"git.erbosoft.com/amy/amsterdam/config"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
)

// amdb is the reference to the Amsterdam database.
var amdb *sqlx.DB

// buildMysqlDSN builds the MySQL DSN for the driver.
func buildMysqlDSN(multiStatement bool) string {
	rc := fmt.Sprintf("%s:%s@tcp(%s)/%s?parseTime=true&loc=UTC",
		config.GlobalComputedConfig.DatabaseUser,
		config.GlobalComputedConfig.DatabasePassword,
		config.GlobalComputedConfig.DatabaseHost,
		config.GlobalComputedConfig.DatabaseName)
	if multiStatement {
		rc += "&multiStatements=true"
	}
	return rc
}

// databaseVersionNumber reads the version number from the database.
func databaseVersionNumber(db *sqlx.DB) (string, error) {
	ver := ""
	err := db.Get(&ver, "SELECT version FROM globals")
	return ver, err
}

// prepareDB prepares the database if it's not yet been loaded.
func prepareDB() (string, error) {
	dsn := buildMysqlDSN(true)
	log.Debugf("dsn=%s", dsn)
	db, err := sqlx.Connect(config.GlobalComputedConfig.DatabaseDriver, dsn)
	if err != nil {
		return "", err
	}
	defer db.Close()
	version, err := databaseVersionNumber(db)
	if err != nil {
		// TODO: database needs initializing here
		log.Errorf("*** cannot get version number: %v", err)
	}
	// TODO: apply migration scripts
	return version, err
}

// SetupDb sets up the database and associated items.
func SetupDb() (func(), error) {
	exitfns := make([]func(), 0, 2)
	version, err := prepareDB()
	if err != nil {
		return nil, err
	}
	db, err := sqlx.Connect(config.GlobalComputedConfig.DatabaseDriver, buildMysqlDSN(false))
	if err == nil {
		amdb = db
		g, err := AmGlobals(context.Background())
		if err == nil {
			if g.Version != version {
				log.Warnf("!! database version %s does not match prepared version %s", g.Version, version)
			}
			setupAdCache()
			setupUserCache()
			setupContactsCache()
			setupCommunityCache()
			setupServicesCache()
			setupConferenceCache()
			exitfns = append(exitfns, setupAuditWriter())
			exitfns = append(exitfns, setupIPBanSweep())
			log.Infof("SetupDb(): database version %s", g.Version)
		}
	}
	slices.Reverse(exitfns)
	return func() {
		for _, f := range exitfns {
			f()
		}
		amdb.Close()
	}, err
}

/* transaction starts a transaction and returns functions for commit and rollback. The rollback
 * function can be immediately deferred; if commit is called successfully, rollback becomes a no-op.
 * Parameters:
 *     ctx - Standard Go error status.
 * Returns:
 *     The sqlx transaction object
 *     The commit function (no parameters, returns error)
 *     The rollback function (no parameters or return)
 */
func transaction(ctx context.Context) (*sqlx.Tx, func() error, func()) {
	tx := amdb.MustBeginTx(ctx, nil)
	live := true
	fCom := func() error {
		var err error = nil
		if live {
			err = tx.Commit()
			if err == nil {
				live = false
			}
		}
		return err
	}
	fRoll := func() {
		if live {
			if err := tx.Rollback(); err != nil {
				log.Errorf("***ROLLBACK ERROR*** %v", err)
			}
			live = false
		}
	}
	return tx, fCom, fRoll
}

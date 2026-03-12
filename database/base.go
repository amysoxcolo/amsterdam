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
func buildMysqlDSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s)/%s?parseTime=true&loc=UTC",
		config.GlobalComputedConfig.DatabaseUser,
		config.GlobalComputedConfig.DatabasePassword,
		config.GlobalComputedConfig.DatabaseHost,
		config.GlobalComputedConfig.DatabaseName)
}

// SetupDb sets up the database and associated items.
func SetupDb() (func(), error) {
	exitfns := make([]func(), 0, 2)
	db, err := sqlx.Connect(config.GlobalComputedConfig.DatabaseDriver, buildMysqlDSN())
	if err == nil {
		amdb = db
		g, err := AmGlobals(context.Background())
		if err == nil {
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
	return func() {
		slices.Reverse(exitfns)
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

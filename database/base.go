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
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"slices"
	"strings"

	"git.erbosoft.com/amy/amsterdam/config"
	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
)

// Error classifications
const (
	classUnspecified = 0
	classNeedInstall = 1
	classNeedConvert = 2
)

// MySQL Errors
var errMySQLNoTable = &mysql.MySQLError{Number: 1146}
var errMySQLNoColumn = &mysql.MySQLError{Number: 1054}

//go:embed mysql-install.sql
var installScriptMySQL string

//go:embed mysql-convert.sql
var convertScriptMySQL string

//go:embed mysql-migrate/*
var migrationsMySQL embed.FS

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

// classifyGetError classifies errors returns from the original get of the version number.
func classifyGetError(err error) int {
	if errors.Is(err, errMySQLNoTable) {
		return classNeedInstall
	}
	if errors.Is(err, errMySQLNoColumn) {
		return classNeedConvert
	}
	return classUnspecified
}

// databaseVersionNumber reads the version number from the database.
func databaseVersionNumber(db *sqlx.DB) (string, error) {
	ver := ""
	err := db.Get(&ver, "SELECT version FROM globals")
	return ver, err
}

// setDatabaseVersionNumber resets the version number in the database.
func setDatabaseVersionNumber(db *sqlx.DB, version string) error {
	_, err := db.Exec("UPDATE globals SET version = ?", version)
	return err
}

// databaseInstallScript returns the install script for the database.
func databaseInstallScript() (string, error) {
	switch config.GlobalComputedConfig.DatabaseDriver {
	case "mysql":
		return installScriptMySQL, nil
	default:
		return "", fmt.Errorf("No install script for database driver: %s", config.GlobalComputedConfig.DatabaseDriver)
	}
}

// databaseConvertScript returns the script to convert a Venice database to Amsterdam.
func databaseConvertScript() (string, error) {
	switch config.GlobalComputedConfig.DatabaseDriver {
	case "mysql":
		return convertScriptMySQL, nil
	default: // N.B.: Not to be implemented for any database type besides MySQL!
		return "", fmt.Errorf("No conversion script for database driver: %s", config.GlobalComputedConfig.DatabaseDriver)
	}
}

// databaseMigrationScripts returns the migration scripts to apply to the database.
func databaseMigrationScripts(version string) (fs.FS, []string, error) {
	var myfs fs.FS
	var err error
	switch config.GlobalComputedConfig.DatabaseDriver {
	case "mysql":
		myfs, err = fs.Sub(migrationsMySQL, "mysql-migrate")
	default:
		err = fmt.Errorf("No migration scripts for database driver: %s", config.GlobalComputedConfig.DatabaseDriver)
	}
	if err != nil {
		return nil, make([]string, 0), err
	}
	rdfs := myfs.(fs.ReadDirFS)
	dents, err := rdfs.ReadDir("/")
	if err != nil {
		return nil, make([]string, 0), err
	}
	rc := make([]string, 0, len(dents))
	for _, d := range dents {
		s := strings.TrimSuffix(d.Name(), ".sql")
		m, err := regexp.Match(`\d{10}`, []byte(s))
		if err != nil {
			return nil, make([]string, 0), err
		}
		if m && s > version {
			rc = append(rc, d.Name())
		}
	}
	if len(rc) > 1 {
		slices.Sort(rc)
	}
	return myfs, rc, nil
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
		switch classifyGetError(err) {
		case classUnspecified:
			log.Errorf("*** cannot get version number: %v (%T)", err, err)
			return version, err
		case classNeedInstall:
			installScript, err := databaseInstallScript()
			if err != nil {
				return "", err
			}
			_, err = db.Exec(installScript)
			if err != nil {
				return "", fmt.Errorf("Failure of install script: %w", err)
			}
		case classNeedConvert:
			convertScript, err := databaseConvertScript()
			if err != nil {
				return "", err
			}
			_, err = db.Exec(convertScript)
			if err != nil {
				return "", fmt.Errorf("Failure of conversion script: %w", err)
			}
		}
		version, err = databaseVersionNumber(db)
		if err != nil {
			return "", err
		}
	}
	scriptfs, scripts, err := databaseMigrationScripts(version)
	if err == nil {
		log.Infof("%d migration script(s) to be applied", len(scripts))
		rffs := scriptfs.(fs.ReadFileFS)
		for _, script := range scripts {
			log.Infof("applying migration script: %s", script)
			var data []byte
			data, err = rffs.ReadFile(script)
			if err != nil {
				return version, fmt.Errorf("Unable to read migration script %s: %w", script, err)
			}
			_, err = db.Exec(string(data))
			if err != nil {
				return version, fmt.Errorf("Unable to apply migration script %s: %w", script, err)
			}
			err = setDatabaseVersionNumber(db, strings.TrimSuffix(script, ".sql"))
			if err != nil {
				break
			}
		}
	}
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

/*
 * Amsterdam Web Communities System
 * Copyright (c) 2025 Erbosoft Metaverse Design Solutions, All Rights Reserved
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

// Package config contains support for Amsterdam site-wide configuration data.
package config

import (
	_ "embed"
	"fmt"
	"os"

	argparse "github.com/alexflint/go-arg"
	"github.com/labstack/gommon/log"
	"gopkg.in/yaml.v3"
)

// AMSTERDAM_VERSION contains the version number of Amsterdam.
const AMSTERDAM_VERSION = "0.0.1"

// AMSTERDAM_COPYRIGHT contains the copyright dates for Amsterdam.
const AMSTERDAM_COPYRIGHT = "2025"

// AmCLI is the command-line interface arguments structure.
type AmCLI struct {
	ConfigFile string `arg:"-C,--config" help:"Location of the configuration file."`
	DebugPanic bool   `arg:"--debug-panic" help:"Development Only - disable Echo panic recovery"`
}

// CommandLine is the command-line arguments passed to Amsterdam.
var CommandLine AmCLI

// Description (from argparse.Described) returns the description string for the application.
func (*AmCLI) Description() string {
	return "Amsterdam Web Communities System Server"
}

// Version (from argparse.Versioned) returns the version number string for the application.
func (*AmCLI) Version() string {
	return "Amsterdam " + AMSTERDAM_VERSION
}

// AmConfig holds the configuration of the application as read from YAML.
type AmConfig struct {
	Site struct {
		BaseURL         string `yaml:"baseURL"`
		Title           string `yaml:"title"`
		TopRefresh      int    `yaml:"topRefresh"`
		LoginCookieName string `yaml:"loginCookieName"`
		LoginCookieAge  int    `yaml:"loginCookieAge"`
		SessionExpire   string `yaml:"sessionExpire"`
		UserAgreement   struct {
			Title string `yaml:"title"`
			Text  string `yaml:"text"`
		} `yaml:"userAgreement"`
	} `yaml:"site"`
	Database struct {
		Driver string `yaml:"driver"`
		Dsn    string `yaml:"dsn"`
	} `yaml:"database"`
	Defaults struct {
		Language string `yaml:"language"`
		TimeZone string `yaml:"timezone"`
	} `yaml:"defaults"`
	Email struct {
		Host         string `yaml:"host"`
		Port         int    `yaml:"port"`
		Tls          string `yaml:"tls"`
		AuthType     string `yaml:"authType"`
		User         string `yaml:"user"`
		Password     string `yaml:"password"`
		MailFromAddr string `yaml:"mailFromAddr"`
		MailFromName string `yaml:"mailFromName"`
		Signature    string `yaml:"signature"`
		Disclaimer   string `yaml:"disclaimer"`
	} `yaml:"email"`
	Rendering struct {
		TemplateDir string `yaml:"templatedir"`
		CookieKey   string `yaml:"cookiekey"`
		CountryList struct {
			Prioritize string `yaml:"prioritize"`
		} `yaml:"countryList"`
	} `yaml:"rendering"`
}

//go:embed default.yaml
var defaultConfigData []byte

// defaultConfig holds the default configuration data.
var defaultConfig AmConfig

// GlobalConfig holds the global configuration.
var GlobalConfig AmConfig

// init prepares the default configuration for the application.
func init() {
	if err := yaml.Unmarshal(defaultConfigData, &defaultConfig); err != nil {
		panic(err) // can't happen
	}
}

/* overlayString is a helper that takes a loaded or defaulted string and returns it.
 * Parameters:
 *     loaded - The string loaded from a configuration file.
 *     defaulted - The default value of this string.
 * Returns:
 *     loaded if it's not empty, otherwise defaulted.
 */
func overlayString(loaded string, defaulted string) string {
	if loaded == "" {
		return defaulted
	}
	return loaded
}

/* overlayInt is a helper that takes a loaded or defaulted integer and returns it.
 * Parameters:
 *     loaded - The integer loaded from a configuration file.
 *     defaulted - The default value of this integer.
 * Returns:
 *     loaded if it's not empty, otherwise defaulted.
 */
func overlayInt(loaded int, defaulted int) int {
	if loaded != 0 {
		return loaded
	}
	return defaulted
}

/* overlayConfig takes two configuration structures and overlays them to create the third.
 * Parameters:
 *     dest - Points to the destination copnfiguration structure.
 *     loaded - Points to the loaded configuration structure.
 *     defaults - Points to the default configuration structure.
 */
func overlayConfig(dest *AmConfig, loaded *AmConfig, defaults *AmConfig) {
	dest.Site.BaseURL = overlayString(loaded.Site.BaseURL, defaults.Site.BaseURL)
	dest.Site.Title = overlayString(loaded.Site.Title, defaults.Site.Title)
	dest.Site.TopRefresh = overlayInt(loaded.Site.TopRefresh, defaults.Site.TopRefresh)
	dest.Site.LoginCookieName = overlayString(loaded.Site.LoginCookieName, defaults.Site.LoginCookieName)
	dest.Site.LoginCookieAge = overlayInt(loaded.Site.LoginCookieAge, defaults.Site.LoginCookieAge)
	dest.Site.SessionExpire = overlayString(loaded.Site.SessionExpire, defaults.Site.SessionExpire)
	dest.Site.UserAgreement.Title = overlayString(loaded.Site.UserAgreement.Title, defaults.Site.UserAgreement.Title)
	dest.Site.UserAgreement.Text = overlayString(loaded.Site.UserAgreement.Text, defaults.Site.UserAgreement.Text)
	dest.Database.Driver = overlayString(loaded.Database.Driver, defaults.Database.Driver)
	dest.Database.Dsn = overlayString(loaded.Database.Dsn, defaults.Database.Dsn)
	dest.Defaults.Language = overlayString(loaded.Defaults.Language, defaults.Defaults.Language)
	dest.Defaults.TimeZone = overlayString(loaded.Defaults.TimeZone, defaults.Defaults.TimeZone)
	dest.Email.Host = overlayString(loaded.Email.Host, defaults.Email.Host)
	dest.Email.Port = overlayInt(loaded.Email.Port, defaults.Email.Port)
	dest.Email.Tls = overlayString(loaded.Email.Tls, defaults.Email.Tls)
	dest.Email.AuthType = overlayString(loaded.Email.AuthType, defaults.Email.AuthType)
	dest.Email.User = overlayString(loaded.Email.User, defaults.Email.User)
	dest.Email.Password = overlayString(loaded.Email.Password, defaults.Email.Password)
	dest.Email.MailFromAddr = overlayString(loaded.Email.MailFromAddr, defaults.Email.MailFromAddr)
	dest.Email.MailFromName = overlayString(loaded.Email.MailFromName, defaults.Email.MailFromName)
	dest.Email.Signature = overlayString(loaded.Email.Signature, defaults.Email.Signature)
	dest.Email.Disclaimer = overlayString(loaded.Email.Disclaimer, defaults.Email.Disclaimer)
	dest.Rendering.TemplateDir = overlayString(loaded.Rendering.TemplateDir, defaults.Rendering.TemplateDir)
	dest.Rendering.CookieKey = overlayString(loaded.Rendering.CookieKey, defaults.Rendering.CookieKey)
	dest.Rendering.CountryList.Prioritize = overlayString(loaded.Rendering.CountryList.Prioritize, defaults.Rendering.CountryList.Prioritize)
}

// SetupConfig loads the command line arguments, loads the config file, and prepares GlobalConfig.
func SetupConfig() {
	argparse.MustParse(&CommandLine)

	if CommandLine.ConfigFile != "" {
		// load the data and use it to unmarshal the loaded configuration
		data, err := os.ReadFile(CommandLine.ConfigFile)
		if err != nil {
			panic(fmt.Sprintf("unable to load configuration file %s: %v", CommandLine.ConfigFile, err))
		}
		var loadedConfig AmConfig
		if err = yaml.Unmarshal(data, &loadedConfig); err != nil {
			panic(fmt.Sprintf("unable to load configuration file %s: %v", CommandLine.ConfigFile, err))
		}
		overlayConfig(&GlobalConfig, &loadedConfig, &defaultConfig)
	} else {
		GlobalConfig = defaultConfig // just copy over the defaults
	}
	log.Infof("Global config: %v", GlobalConfig)
}

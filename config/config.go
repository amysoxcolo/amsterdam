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
		Title string `yaml:"title"`
	} `yaml:"site"`
	Database struct {
		Driver string `yaml:"driver"`
		Dsn    string `yaml:"dsn"`
	} `yaml:"database"`
	Rendering struct {
		TemplateDir string `yaml:"templatedir"`
		CookieKey   string `yaml:"cookiekey"`
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

/* overlayConfig takes two configuration structures and overlays them to create the third.
 * Parameters:
 *     dest - Points to the destination copnfiguration structure.
 *     loaded - Points to the loaded configuration structure.
 *     defaults - Points to the default configuration structure.
 */
func overlayConfig(dest *AmConfig, loaded *AmConfig, defaults *AmConfig) {
	dest.Site.Title = overlayString(loaded.Site.Title, defaults.Site.Title)
	dest.Rendering.TemplateDir = overlayString(loaded.Rendering.TemplateDir, defaults.Rendering.TemplateDir)
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

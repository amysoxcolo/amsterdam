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

	"gopkg.in/yaml.v3"
)

// AmConfig holds the configuration of the application as read from YAML.
type AmConfig struct {
	Site struct {
		Title string `yaml:"title"`
	} `yaml:"site"`
	Rendering struct {
		TemplateDir string `yaml:"templatedir"`
	} `yaml:"rendering"`
}

//go:embed default.yaml
var defaultConfigData []byte

// GlobalConfig holds the global configuration.
var GlobalConfig AmConfig

// init prepares the default configuration for the application.
func init() {
	var defaultConfig AmConfig
	if err := yaml.Unmarshal(defaultConfigData, &defaultConfig); err != nil {
		panic(err) // can't happen
	}
	GlobalConfig = defaultConfig
}

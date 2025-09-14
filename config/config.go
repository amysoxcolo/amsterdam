/*
 * Amsterdam Web Communities System
 * Copyright (c) 2025 Erbosoft Metaverse Design Solutions, All Rights Reserved
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */
package config

import (
	_ "embed"

	"gopkg.in/yaml.v3"
)

type AmConfig struct {
	Site struct {
		Title string `yaml:"title"`
	} `yaml:"site"`
}

//go:embed default.yaml
var defaultConfigData []byte

var GlobalConfig AmConfig

func init() {
	var defaultConfig AmConfig
	yaml.Unmarshal(defaultConfigData, &defaultConfig)
	GlobalConfig = defaultConfig
}

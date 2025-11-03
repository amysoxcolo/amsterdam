/*
 * Amsterdam Web Communities System
 * Copyright (c) 2025 Erbosoft Metaverse Design Solutions, All Rights Reserved
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */
// The htmlcheck package contains the HTML Checker.
package htmlcheck

import (
	_ "embed"

	"gopkg.in/yaml.v3"
)

// HTMLCheckerConfig is a configuration that may be used with the HTML Checker.
type HTMLCheckerConfig struct {
	Name             string   `yaml:"name"`
	WordWrap         int      `yaml:"wordWrap"`
	Rewrap           bool     `yaml:"rewrap"`
	Angles           bool     `yaml:"angles"`
	Parens           bool     `yaml:"parens"`
	DiscardHTML      bool     `yaml:"discardHTML"`
	DiscardRejected  bool     `yaml:"discardRejected"`
	DiscardComments  bool     `yaml:"discardComments"`
	DiscardXML       bool     `yaml:"discardXML"`
	OutputFilters    []string `yaml:"outputFilters"`
	RawOutputFilters []string `yaml:"rawOutputFilters"`
	StringRewriters  []string `yaml:"stringRewriters"`
	WordRewriters    []string `yaml:"wordRewriters"`
	TagRewriters     []string `yaml:"tagRewriters"`
	ParenRewriters   []string `yaml:"parenRewriters"`
	TagSet           string   `yaml:"tagSet"`
	DisallowTags     []string `yaml:"disallowTags"`
	AnchorTail       string   `yaml:"anchorTail"`
}

// HTMLCheckerConfigFile represents all the configs as they exist in the file.
type HTMLCheckerConfigFile struct {
	Configs []HTMLCheckerConfig `yaml:"configs"`
}

const defaultAnchorTail = "TARGET=\"Wander\""

//go:embed configs.yaml
var configData []byte

// configsRegistry contains all the known configurations by name.
var configsRegistry = make(map[string]*HTMLCheckerConfig)

// init loads the configuration data.
func init() {
	var cfgdata HTMLCheckerConfigFile
	err := yaml.Unmarshal(configData, &cfgdata)
	if err != nil {
		panic(err)
	}
	for i := range cfgdata.Configs {
		configsRegistry[cfgdata.Configs[i].Name] = &(cfgdata.Configs[i])
		if cfgdata.Configs[i].AnchorTail == "" {
			cfgdata.Configs[i].AnchorTail = defaultAnchorTail
		}
	}
}

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

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// HTMLCheckerConfig is a configuration that may be used with the HTML Checker.
type HTMLCheckerConfig struct {
	Name            string   `yaml:"name"`
	WordWrap        int      `yaml:"wordWrap"`
	Rewrap          bool     `yaml:"rewrap"`
	Angles          bool     `yaml:"angles"`
	Parens          bool     `yaml:"parens"`
	DiscardHTML     bool     `yaml:"discardHTML"`
	DiscardRejected bool     `yaml:"discardRejected"`
	DiscardComments bool     `yaml:"discardComments"`
	DiscardXML      bool     `yaml:"discardXML"`
	OutputFilters   []string `yaml:"outputFilters"`
	StringRewriters []string `yaml:"stringRewriters"`
	WordRewriters   []string `yaml:"wordRewriters"`
	TagRewriters    []string `yaml:"tagRewriters"`
	ParenRewriters  []string `yaml:"parenRewriters"`
	TagSet          string   `yaml:"tagSet"`
	DisallowTags    []string `yaml:"disallowTags"`
}

func (cfg *HTMLCheckerConfig) rezOutputFilters() []outputFilter {
	rc := make([]outputFilter, 0, len(cfg.OutputFilters))
	for i := range cfg.OutputFilters {
		f, ok := outputFilterRegistry[cfg.OutputFilters[i]]
		if ok {
			rc = append(rc, f)
		} else {
			log.Errorf("filter %s is not found", cfg.OutputFilters[i])
		}
	}
	return rc
}

func rezRewriters(desired []string) []rewriter {
	rc := make([]rewriter, 0, len(desired))
	for i := range desired {
		r, ok := rewriterRegistry[desired[i]]
		if ok {
			rc = append(rc, r)
		} else {
			log.Errorf("rewriter %s is not found", desired[i])
		}
	}
	return rc
}

func (cfg *HTMLCheckerConfig) rezStringRewriters() []rewriter {
	return rezRewriters(cfg.StringRewriters)
}

func (cfg *HTMLCheckerConfig) rezWordRewriters() []rewriter {
	return rezRewriters(cfg.WordRewriters)
}

func (cfg *HTMLCheckerConfig) rezTagRewriters() []rewriter {
	return rezRewriters(cfg.TagRewriters)
}

func (cfg *HTMLCheckerConfig) rezParenRewriters() []rewriter {
	return rezRewriters(cfg.ParenRewriters)
}

// HTMLCheckerConfigFile represents all the configs as they exist in the file.
type HTMLCheckerConfigFile struct {
	Configs []HTMLCheckerConfig `yaml:"configs"`
}

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
	}
}

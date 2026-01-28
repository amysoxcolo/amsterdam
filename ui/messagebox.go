/*
 * Amsterdam Web Communities System
 * Copyright (c) 2025-2026 Erbosoft Metaverse Design Solutions, All Rights Reserved
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */
// Package ui holds the support for the Amsterdam user interface, wrapping Echo and Jet templates.
package ui

import (
	_ "embed"

	"gopkg.in/yaml.v3"
)

// MBoxWarningLine defines a single warning line in a message box.
type MBoxWarningLine struct {
	Text string `yaml:"text"`
	Bold bool   `yaml:"bold"`
}

// MBoxButton defines a single button on a message box.
type MBoxButton struct {
	Id      string `yaml:"id"`
	Link    string `yaml:"link"`
	Confirm bool   `yaml:"confirm"`
	Tone    string `yaml:"tone"`
	Icon    string `yaml:"icon"`
	Text    string `yaml:"text"`
}

// MessageBoxDefinition defines a single message box resource.
type MessageBoxDefinition struct {
	Id           string            `yaml:"id"`
	Title        string            `yaml:"title"`
	Tone         string            `yaml:"tone"`
	Destructive  bool              `yaml:"destructive"`
	Message      string            `yaml:"message"`
	WarningIcon  string            `yaml:"warningIcon"`
	WarningLines []MBoxWarningLine `yaml:"warningLines"`
	Buttons      []MBoxButton      `yaml:"buttons"`
}

// MessageBoxDefs is the top-level structure for defining message boxes.
type MessageBoxDefs struct {
	D []MessageBoxDefinition `yaml:"messagedefs"`
}

//go:embed messagedefs.yaml
var initMessageData []byte

// messageBoxDefs is the master repository for message box data.
var messageBoxDefs MessageBoxDefs

// init loads and binds the message box definitions.
func init() {
	if err := yaml.Unmarshal(initMessageData, &messageBoxDefs); err != nil {
		panic(err) // can't happen
	}
}

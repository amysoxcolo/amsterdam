/*
 * Amsterdam Web Communities System
 * Copyright (c) 2025 Erbosoft Metaverse Design Solutions, All Rights Reserved
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

// Package ui holds the support for the Amsterdam user interface, wrapping Echo and Jet templates.
package ui

import (
	"embed"
	"fmt"

	"gopkg.in/yaml.v3"
)

// DialogItem holds the dialog item definition.
type DialogItem struct {
	Type      string `yaml:"type"`
	Name      string `yaml:"name"`
	Caption   string `yaml:"caption,omitempty"`
	Size      int    `yaml:"size,omitempty"`
	MaxLength int    `yaml:"maxlength,omitempty"`
	Value     string `yaml:"value,omitempty"`
	Tone      string `yaml:"tone,omitempty"`
}

// Dialog holds the dialog definition.
type Dialog struct {
	Name         string       `yaml:"name"`
	FormName     string       `yaml:"formName"`
	MenuSelector string       `yaml:"menuSelector,omitempty"`
	Title        string       `yaml:"title"`
	Action       string       `yaml:"action"`
	Instructions string       `yaml:"instructions,omitempty"`
	Fields       []DialogItem `yaml:"fields"`
}

//go:embed dialogs/*
var dialogs embed.FS

/* AmLoadDialog loads a dialog definition.
 * Parameters:
 *     name - The name of the dialog to load
 */
func AmLoadDialog(name string) (*Dialog, error) {
	b, err := dialogs.ReadFile(fmt.Sprintf("dialogs/%s.yaml", name))
	if err == nil {
		var d Dialog
		err = yaml.Unmarshal(b, &d)
		if err == nil {
			if d.MenuSelector == "" {
				d.MenuSelector = "nochange"
			}
			return &d, nil
		}
	}
	return nil, err
}

/* Render sets up the rendering parameters to send this dialog to the output.
 * Parameters:
 *     ctxt - The AmContext for this request.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func (d *Dialog) Render(ctxt AmContext) (string, any, error) {
	ctxt.VarMap().Set("amsterdam_dialog", d)
	return "framed_template", "dialog.jet", nil
}

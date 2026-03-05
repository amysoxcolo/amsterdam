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
	"crypto/sha1"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"

	"git.erbosoft.com/amy/amsterdam/config"
	"git.erbosoft.com/amy/amsterdam/util"
	log "github.com/sirupsen/logrus"
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
	useConfirm   bool
}

// MessageBoxDefs is the top-level structure for defining message boxes.
type MessageBoxDefs struct {
	D     []MessageBoxDefinition `yaml:"messagedefs"`
	table map[string]*MessageBoxDefinition
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
	messageBoxDefs.table = make(map[string]*MessageBoxDefinition)
	for i, def := range messageBoxDefs.D {
		messageBoxDefs.table[def.Id] = &(messageBoxDefs.D[i])
		messageBoxDefs.D[i].useConfirm = false
		for _, b := range messageBoxDefs.D[i].Buttons {
			if b.Confirm {
				messageBoxDefs.D[i].useConfirm = true
				break
			}
		}
	}
}

// setupMessageBoxes loads external message box definitions.
func setupMessageBoxes() {
	if config.GlobalConfig.Resources.ExternalMessageDefinitions != "" {
		b, err := os.ReadFile(config.GlobalConfig.Resources.ExternalMessageDefinitions)
		if err == nil {
			mb := new(MessageBoxDefs)
			err = yaml.Unmarshal(b, mb)
			if err == nil {
				for i, def := range mb.D {
					messageBoxDefs.table[def.Id] = &(mb.D[i])
					mb.D[i].useConfirm = false
					for _, b := range mb.D[i].Buttons {
						if b.Confirm {
							mb.D[i].useConfirm = true
							break
						}
					}
				}
			} else {
				log.Errorf("cannot parse external message definition file %s, ignored (%v)", config.GlobalConfig.Resources.ExternalMessageDefinitions, err)
			}
		} else {
			log.Errorf("cannot read external message definition file %s, ignored (%v)", config.GlobalConfig.Resources.ExternalMessageDefinitions, err)
		}
	}
}

// MessageBox is the structure for a working message box.
type MessageBox struct {
	def         *MessageBoxDefinition
	message     string
	buttonLinks []string
}

// SetMessage sets the actual message inside the message box.
func (mb *MessageBox) SetMessage(t string) {
	mb.message = t
}

// SetLink sets the link for a specific button in the box.
func (mb *MessageBox) SetLink(id, link string) {
	for i := range mb.def.Buttons {
		if mb.def.Buttons[i].Id == id {
			mb.buttonLinks[i] = link
			break
		}
	}
}

// Render sets up to render the message box.
func (mb *MessageBox) Render(ctxt AmContext) (string, any) {
	blinks := mb.buttonLinks
	if mb.def.useConfirm {
		nonce := util.GenerateRandomAuthString()
		blinks = make([]string, len(mb.buttonLinks))
		for i := range mb.buttonLinks {
			if mb.def.Buttons[i].Confirm {
				hasher := sha1.New()
				hasher.Write([]byte(mb.def.Buttons[i].Id))
				confirmString := hex.EncodeToString(hasher.Sum([]byte(nonce)))
				if strings.Contains(mb.buttonLinks[i], "?") {
					blinks[i] = fmt.Sprintf("%s&confirm=%s", mb.buttonLinks[i], confirmString)
				} else {
					blinks[i] = fmt.Sprintf("%s?confirm=%s", mb.buttonLinks[i], confirmString)
				}
			} else {
				blinks[i] = mb.buttonLinks[i]
			}
		}
		ctxt.SetSession("mbconfirm."+mb.def.Id, nonce)
	}
	ctxt.SetFrameTitle(mb.def.Title)
	ctxt.VarMap().Set("tone", mb.def.Tone)
	ctxt.VarMap().Set("destructive", mb.def.Destructive)
	ctxt.VarMap().Set("message", mb.message)
	ctxt.VarMap().Set("useWarning", len(mb.def.WarningIcon) > 0 && len(mb.def.WarningLines) > 0)
	ctxt.VarMap().Set("warningIcon", mb.def.WarningIcon)
	ctxt.VarMap().Set("warningLines", mb.def.WarningLines)
	ctxt.VarMap().Set("buttons", mb.def.Buttons)
	ctxt.VarMap().Set("buttonLinks", blinks)
	return "framed", "messagebox.jet"
}

// Validate validates that the correct button was clicked by verifying the confirmation parameter.
func (mb *MessageBox) Validate(ctxt AmContext, buttonid string) bool {
	var nonceAny any
	nonceAny = ctxt.GetSession("mbconfirm." + mb.def.Id)
	ctxt.SetSession("mbconfirm."+mb.def.Id, "")
	if nonce, ok := nonceAny.(string); ok {
		confirm := ctxt.Parameter("confirm")
		hasher := sha1.New()
		hasher.Write([]byte(buttonid))
		confirmString := hex.EncodeToString(hasher.Sum([]byte(nonce)))
		if confirm == confirmString {
			return true
		}
	}
	return false
}

// AmLoadMessageBox loads a message box structure by ID.
func AmLoadMessageBox(id string) (*MessageBox, error) {
	if def, ok := messageBoxDefs.table[id]; ok {
		mbox := MessageBox{
			def:         def,
			message:     def.Message,
			buttonLinks: make([]string, len(def.Buttons)),
		}
		for i := range def.Buttons {
			mbox.buttonLinks[i] = def.Buttons[i].Link
		}
		return &mbox, nil
	}
	return nil, errors.New("message box not found")
}

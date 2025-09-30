/*
 * Amsterdam Web Communities System
 * Copyright (c) 2025 Erbosoft Metaverse Design Solutions, All Rights Reserved
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

// Package email contains support for E-mail messages sent by Amsterdam.
package email

import "fmt"

// Message is the interface for an E-mail message to be sent.
type Message interface {
	SetFrom(string, string)
	AddTo(string, string)
	AddCC(string, string)
	AddBCC(string, string)
	SetSubject(string)
	SetText(string)
	AddHeader(string, string)
}

type amMessage struct {
	from    string
	to      []string
	cc      []string
	bcc     []string
	subject string
	text    string
	headers map[string]string
}

func formatAddress(addr string, name string) string {
	if name == "" {
		return addr
	} else {
		return fmt.Sprintf("%s <%s>", name, addr)
	}
}

func (m *amMessage) SetFrom(addr string, name string) {
	m.from = formatAddress(addr, name)
}

func (m *amMessage) AddTo(addr string, name string) {
	m.to = append(m.to, formatAddress(addr, name))
}

func (m *amMessage) AddCC(addr string, name string) {
	m.cc = append(m.cc, formatAddress(addr, name))
}

func (m *amMessage) AddBCC(addr string, name string) {
	m.bcc = append(m.bcc, formatAddress(addr, name))
}

func (m *amMessage) SetSubject(s string) {
	m.subject = s
}

func (m *amMessage) SetText(txt string) {
	m.text = txt
}

func (m *amMessage) AddHeader(name string, value string) {
	m.headers[name] = value
}

func AmNewEmailMessage() Message {
	rc := amMessage{to: make([]string, 0), cc: make([]string, 0), bcc: make([]string, 0), headers: make(map[string]string)}
	return &rc
}

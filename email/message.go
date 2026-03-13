/*
 * Amsterdam Web Communities System
 * Copyright (c) 2025 Erbosoft Metaverse Design Solutions, All Rights Reserved
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * SPDX-License-Identifier: MPL-2.0
 */

// Package email contains support for E-mail messages sent by Amsterdam.
package email

import (
	"fmt"
	"sync"

	"git.erbosoft.com/amy/amsterdam/config"
	"github.com/CloudyKit/jet/v6"
)

// Message is the interface for an E-mail message to be sent.
type Message interface {
	SetFrom(string, string)
	AddTo(string, string)
	AddCC(string, string)
	AddBCC(string, string)
	GetSubject() string
	SetSubject(string)
	SetText(string)
	AddHeader(string, string)
	SetTemplate(string)
	AddVariable(string, any)
	Send()
}

// amMessage is the internal structure of the Message.
type amMessage struct {
	from     string
	fromAddr string
	to       []string
	toAddrs  []string
	cc       []string
	bcc      []string
	subject  string
	text     string
	headers  map[string]string
	template string
	vars     jet.VarMap
	uid      int32
	ip       string
}

// freeMessages is a free list for amMessage structures.
var freeMessages sync.Pool

// formatAddress outputs an E-mail address with optional name associated with it.
func formatAddress(addr string, name string) string {
	if name == "" {
		return addr
	} else {
		return fmt.Sprintf("%s <%s>", name, addr)
	}
}

// SetFrom sets the From: address of the message.
func (m *amMessage) SetFrom(addr string, name string) {
	m.from = formatAddress(addr, name)
	m.fromAddr = addr
}

// AddTo ads a To: address to the message.
func (m *amMessage) AddTo(addr string, name string) {
	m.to = append(m.to, formatAddress(addr, name))
	m.toAddrs = append(m.toAddrs, addr)
}

// AddCC ads a Cc: address to the message.
func (m *amMessage) AddCC(addr string, name string) {
	m.cc = append(m.cc, formatAddress(addr, name))
	m.toAddrs = append(m.toAddrs, addr)
}

// AddBCC ads a Bcc: address to the message.
func (m *amMessage) AddBCC(addr string, name string) {
	m.bcc = append(m.bcc, formatAddress(addr, name))
	m.toAddrs = append(m.toAddrs, addr)
}

// GetSubject gets the message's subject.
func (m *amMessage) GetSubject() string {
	return m.subject
}

// SetSubject sets the message's subject.
func (m *amMessage) SetSubject(s string) {
	m.subject = s
}

// SetText sets the text of the message.
func (m *amMessage) SetText(txt string) {
	m.text = txt
}

// AddHaader adds a new header to the message.
func (m *amMessage) AddHeader(name string, value string) {
	m.headers[name] = value
}

func (m *amMessage) SetTemplate(templ string) {
	m.template = templ
}

func (m *amMessage) AddVariable(name string, value any) {
	m.vars.Set(name, value)
}

func (m *amMessage) Send() {
	sendChan <- m
}

/* AmNewEmailMessage creates a new message and returns it.
 * Parameters:
 *     sender = User ID of the person sending the message.
 *     ip = IP address of the person sending the message.
 * Returns:
 *     The new Message.
 */
func AmNewEmailMessage(sender int32, ip string) Message {
	var rc *amMessage
	tmp := freeMessages.Get()
	if tmp == nil {
		rc = &amMessage{to: make([]string, 0), cc: make([]string, 0), bcc: make([]string, 0),
			headers: make(map[string]string), vars: make(jet.VarMap)}
	} else {
		rc = tmp.(*amMessage)
	}
	rc.uid = sender
	rc.ip = ip
	rc.SetFrom(config.GlobalConfig.Email.MailFromAddr, config.GlobalConfig.Email.MailFromName)
	return rc
}

// The "recycle bin" for messages.
var messageRecycleBin chan *amMessage

// recycleMessages is a goroutine that recycles the messages on its queue.
func recycleMessages(messages chan *amMessage, done chan bool) {
	for m := range messages {
		m.from = ""
		m.fromAddr = ""
		m.to = make([]string, 0)
		m.toAddrs = make([]string, 0)
		m.cc = make([]string, 0)
		m.bcc = make([]string, 0)
		m.subject = ""
		m.text = ""
		clear(m.headers)
		m.template = ""
		clear(m.vars)
		freeMessages.Put(m)
	}
	done <- true
}

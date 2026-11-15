/*
 * Amsterdam Web Communities System
 * Copyright (c) 2025-2026 Erbosoft Metaverse Design Solutions, All Rights Reserved
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * SPDX-License-Identifier: MPL-2.0
 */

// Package agentclass contains support for agent request classification.
package agentclass

import (
	_ "embed"
	"encoding/json/v2"
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

// RawAgentDef is the agent definition data loaded from the agents.json file.
type RawAgentDef struct {
	Id             string   `json:"id"`
	Name           string   `json:"name"`
	Operator       string   `json:"operator"`
	Category       string   `json:"category"`
	Kind           string   `json:"kind"`
	RunsOn         string   `json:"runsOn"`
	UserAgents     []string `json:"userAgents"`
	Accept         []string `json:"accept"`
	SignatureAgent []string `json:"signatureAgent"`
	Verify         struct {
		IpRanges   []string `json:"ipRanges"`
		ReverseDNS []string `json:"reverseDns"`
		WebBotAuth string   `json:"webBotAuth"`
	} `json:"verify"`
	Docs         string   `json:"docs"`
	Examples     []string `json:"examples"`
	Notes        string   `json:"notes"`
	LastVerified string   `json:"lastVerified"`
}

// agentDef is the definition for a compiled agent.
type agentDef struct {
	r        *RawAgentDef     // all basic agent information
	patterns []*regexp.Regexp // compiled user agent expressions
}

//go:embed agents.json
var initAgents []byte

// compiledAgents is the compiled list of agents.
var compiledAgents []agentDef

func AmInitAgentClass() {
	var rawAgents []RawAgentDef
	err := json.Unmarshal(initAgents, &rawAgents)
	if err != nil {
		panic(fmt.Sprintf("agent load failure: %v", err))
	}
	compiledAgents = make([]agentDef, len(rawAgents))
	for i, v := range rawAgents {
		compiledAgents[i].r = &(rawAgents[i])
		compiledAgents[i].patterns = make([]*regexp.Regexp, len(v.UserAgents))
		for j, p := range v.UserAgents {
			compiledAgents[i].patterns[j] = regexp.MustCompile("(?i)" + p)
		}
	}
}

func normalizeHeaders(headers http.Header) map[string]string {
	rc := make(map[string]string)
	if headers == nil {
		return rc
	}
	for k, v := range headers {
		rc[strings.ToLower(k)] = strings.Join(v, ", ")
	}
	return rc
}

func findAgent(ua string, h map[string]string) {
	for _, ca := range compiledAgents {
		for _, p := range ca.patterns {

		}
	}
}

func AmClassify(headers http.Header, userAgent string) {
	h := normalizeHeaders(headers)
	ua := userAgent
	if ua == "" {
		t, ok := h["user-agent"]
		if ok {
			ua = t
		}
	}
	findAgent(ua, h)
}

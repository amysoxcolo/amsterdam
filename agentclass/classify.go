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
	"slices"
	"strings"
)

const (
	ConfidenceHigh = iota
	ConfidenceMedium
	ConfidenceLow
)

type Classification struct {
	Category   string
	Kind       string
	Id         string
	Name       string
	Operator   string
	Version    string
	Verified   bool
	Confidence int
	Reasons    []string
}

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
		if v.UserAgents != nil {
			compiledAgents[i].patterns = make([]*regexp.Regexp, len(v.UserAgents))
			for j, p := range v.UserAgents {
				compiledAgents[i].patterns[j] = regexp.MustCompile("(?i)" + p)
			}
		} else {
			compiledAgents[i].patterns = make([]*regexp.Regexp, 0)
		}
	}
}

type AgentMatch struct {
	Agent   *RawAgentDef
	Version string
	Reason  string
	Weak    bool
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

var normalizeAcceptExpr = regexp.MustCompile(`\s*([,;=])\s*`)

func normalizeAccept(accept string) string {
	return strings.TrimSpace(normalizeAcceptExpr.ReplaceAllString(strings.ToLower(accept), "$1"))
}

var signatureAgentHostExpr = regexp.MustCompile(`(?i)https?:\/\/([^/"\s]+)`)
var signatureAgentHostBare1 = regexp.MustCompile(`^[\w-]+=`)
var signatureAgentHostBare2 = regexp.MustCompile(`"`)

func signatureAgentHost(value string) *string {
	m := signatureAgentHostExpr.FindStringSubmatch(value)
	if m != nil {
		return new(strings.ToLower(m[1]))
	}
	bare := strings.TrimSpace(signatureAgentHostBare2.ReplaceAllString(signatureAgentHostBare1.ReplaceAllString(value, ""), ""))
	if bare != "" {
		return new(strings.ToLower(bare))
	}
	return nil
}

func findAgent(ua string, h map[string]string) *AgentMatch {
	for _, ca := range compiledAgents {
		for _, p := range ca.patterns {
			m := p.FindStringSubmatch(ua)
			if m != nil {
				ndx := p.SubexpIndex("version")
				vers := ""
				if ndx > 0 {
					vers = m[ndx]
				}
				return new(AgentMatch{Agent: ca.r, Version: vers, Reason: fmt.Sprintf("user agent matches %s", ca.r.Name), Weak: false})
			}
		}
	}
	accept, ok := h["accept"]
	if ok {
		accept = normalizeAccept(accept)
		p := slices.IndexFunc(compiledAgents[:], func(ad agentDef) bool {
			if ad.r.Accept == nil {
				return false
			}
			for _, t := range ad.r.Accept {
				if normalizeAccept(t) == accept {
					return true
				}
			}
			return false
		})
		if p >= 0 {
			rc := compiledAgents[p]
			return new(AgentMatch{Agent: rc.r, Version: "", Reason: fmt.Sprintf("Accept header matches %s's fingerprint", rc.r.Name), Weak: false})
		}
	}
	_, isSigned := h["signed"]
	if isSigned {
		_, isSigned = h["signature-input"]
	}
	if isSigned {
		sig, ok := h["signature-agent"]
		if ok {
			psig := signatureAgentHost(sig)
			if psig != nil {
				p := slices.IndexFunc(compiledAgents[:], func(ad agentDef) bool {
					if ad.r.SignatureAgent == nil {
						return false
					}
					return slices.Index(ad.r.SignatureAgent[:], *psig) >= 0
				})
				if p >= 0 {
					rc := compiledAgents[p]
					return new(AgentMatch{Agent: rc.r, Version: "",
						Reason: fmt.Sprintf("signed request names %s in Signature-Agent; the signature itself is not verified", *psig),
						Weak:   true})
				}
			}
		}
	}
	return nil
}

func fromAgent(match *AgentMatch, rawIP string) Classification {
	agent := match.Agent
	base := Classification{
		Category:   agent.Category,
		Kind:       agent.Kind,
		Name:       agent.Name,
		Operator:   agent.Operator,
		Version:    match.Version,
		Verified:   true,
		Confidence: ConfidenceMedium,
		Reasons:    make([]string, 1),
	}
	base.Reasons[0] = match.Reason
	if agent.RunsOn == "user" {
		base.Reasons = append(base.Reasons, fmt.Sprintf("%s runs on the user's machine, so IP verification does not apply", agent.Name))
		return base
	}
	ip := normalizeIP(rawIP)
	if ip == nil {
		base.Reasons = append(base.Reasons, fmt.Sprintf("%s is not a single valid address, so it was not checked", rawIP))
		return base
	}
	urls := make([]string, 0)
	if agent.Verify.IpRanges != nil {
		urls = append(urls, agent.Verify.IpRanges...)
	}
}

func AmClassify(headers http.Header, ipAddr string, userAgent string) Classification {
	h := normalizeHeaders(headers)
	ua := userAgent
	if ua == "" {
		t, ok := h["user-agent"]
		if ok {
			ua = t
		}
	}
	match := findAgent(ua, h)
	if match != nil {
		return fromAgent(match, ipAddr)
	}
}

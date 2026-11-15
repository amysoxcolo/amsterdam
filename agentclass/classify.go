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
	"maps"
	"net/http"
	"regexp"
	"slices"
	"strings"
)

// Constants reflecting confidence levels.
const (
	ConfidenceHigh   = iota // high confidence
	ConfidenceMedium        // medium confidence
	ConfidenceLow           // low confidence
)

// Classification is the return value from AmClassify, showing what a particular HTTP request is classified as.
type Classification struct {
	Category   string   // category of requester: agent, ai-crawler, crawler, bot, human, unknown
	Kind       string   // what kind of requester is it?
	Id         string   // actual identifier
	Name       string   // name of requester
	Operator   string   // what organization operates this requester?
	Version    string   // version of requester, where applicable
	Verified   bool     // true if all this is verified, false if not
	Confidence int      // confidence level (above)
	Reasons    []string // why did we classify the request this way?
}

// RawAgentDef is the agent definition data loaded from the agents.json file.
type RawAgentDef struct {
	Id             string   `json:"id"`             // agent ID
	Name           string   `json:"name"`           // agent name
	Operator       string   `json:"operator"`       // agent operating organization
	Category       string   `json:"category"`       // category of agent
	Kind           string   `json:"kind"`           // kind (sub-category) of agent
	RunsOn         string   `json:"runsOn"`         // does it run on a user machine or an operator machine?
	UserAgents     []string `json:"userAgents"`     // regexps to recognize its User-Agent
	Accept         []string `json:"accept"`         // Accept: string(s) the agent sends
	SignatureAgent []string `json:"signatureAgent"` // what agent(s) we can find in a request signature
	Verify         struct {
		IpRanges   []string `json:"ipRanges"`   // where can we find what IP ranges this agent uses?
		ReverseDNS []string `json:"reverseDns"` // what should the agent's IP address resolve to?
		WebBotAuth string   `json:"webBotAuth"` // where can I find the authentication?
	} `json:"verify"`
	Docs         string   `json:"docs"`         // URL to documentation
	Examples     []string `json:"examples"`     // examples of strings that match
	Notes        string   `json:"notes"`        // notes
	LastVerified string   `json:"lastVerified"` // when was this information last verified?
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

// AmInitAgentClass is the initialization function.
func AmInitAgentClass() {
	// load the raw agent list
	var rawAgents []RawAgentDef
	err := json.Unmarshal(initAgents, &rawAgents)
	if err != nil {
		panic(fmt.Sprintf("agent load failure: %v", err))
	}

	// build the compiled agent list
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

	// get the list of IP range files
	urls := make(map[string]struct{})
	for _, ra := range rawAgents {
		if ra.Verify.IpRanges != nil {
			for _, url := range ra.Verify.IpRanges {
				urls[url] = struct{}{}
			}
		}
	}
	loadIPRanges(slices.Collect(maps.Keys(urls)))
}

// AgentMatch is the return from findAgent.
type AgentMatch struct {
	Agent   *RawAgentDef // pointer to raw agent definition
	Version string       // version of agent, where applicable
	Reason  string       // reason we matched this agent
	Weak    bool         // is this a weak match?
}

// normalizeHeaders joins header values and smashes case on header names.
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

// normalizeAcceptExpr is how we close the gaps around delimiters in an Accept: expression.
var normalizeAcceptExpr = regexp.MustCompile(`\s*([,;=])\s*`)

// normalizeAccept squeezes the whitespace out of an Accept: expression.
func normalizeAccept(accept string) string {
	return strings.TrimSpace(normalizeAcceptExpr.ReplaceAllString(strings.ToLower(accept), "$1"))
}

// signatureAgentHostExpr recognizes the "host" in a URL.
var signatureAgentHostExpr = regexp.MustCompile(`(?i)https?:\/\/([^/"\s]+)`)

// signatureAgentHostBare1 is used to trim up a bare host name.
var signatureAgentHostBare1 = regexp.MustCompile(`^[\w-]+=`)

// signatureAgentHostBare2 is used to trim up a bare host name.
var signatureAgentHostBare2 = regexp.MustCompile(`"`)

// signatureAgentHost locates the host associated with a request signature.
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

// findAgent matches the request against the list of compiled agents to locate a match. Returns nil on failure.
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

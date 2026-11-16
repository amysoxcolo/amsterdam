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
/* This file contains code adapted from Mintlify agent-traffic, https://github.com/mintlify/agent-traffic
 * ---
 * Copyright (c) 2026 Mintlify, Inc.
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 * SOFTWARE.
 */

// Package agentclass contains support for agent request classification.
package agentclass

import (
	_ "embed"
	"encoding/json/v2"
	"fmt"
	"maps"
	"math"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
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

// RawClientDef is the client definition data loaded from the http-clients.json file.
type RawClientDef struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	UserAgent string `json:"userAgent"`
}

// clientdef is the definition for a compiled client.
type clientDef struct {
	r       *RawClientDef
	pattern *regexp.Regexp
}

//go:embed agents.json
var initAgents []byte

// compiledAgents is the compiled list of agents.
var compiledAgents []agentDef

//go:embed http-clients.json
var initClients []byte

// compiledClients is the compiled list of clients.
var compiledClients []clientDef

// AmInitAgentClass is the initialization function.
func AmInitAgentClass(ctxname any) func() {
	// load the raw agent list
	var rawAgents []RawAgentDef
	err := json.Unmarshal(initAgents, &rawAgents)
	if err != nil {
		panic(fmt.Sprintf("agent definition load failure: %v", err))
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

	// load the raw clients list
	var rawClients []RawClientDef
	err = json.Unmarshal(initClients, &rawClients)
	if err != nil {
		panic(fmt.Sprintf("client definition load failure: %v", err))
	}

	// build the compiled client list
	compiledClients = make([]clientDef, len(rawClients))
	for i, v := range rawClients {
		compiledClients[i].r = &(rawClients[i])
		compiledClients[i].pattern = regexp.MustCompile("(?i)" + v.UserAgent)
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
	return loadIPRanges(slices.Collect(maps.Keys(urls)), ctxname)
}

type ClassifyOptions struct {
	MaxAge          int        // maximum age of IP range files
	Now             *time.Time // "now" timestamp (for testing)
	HeadersComplete bool       // look at complete headers?
}

func AmClassifyOptions() ClassifyOptions {
	return ClassifyOptions{
		MaxAge:          180,
		Now:             nil,
		HeadersComplete: false,
	}
}

func (o *ClassifyOptions) WithMaxAge(maxAge int) *ClassifyOptions {
	o.MaxAge = maxAge
	return o
}

func (o *ClassifyOptions) WithNow(now *time.Time) *ClassifyOptions {
	o.Now = now
	return o
}

func (o *ClassifyOptions) WithHeadersComplete(flag bool) *ClassifyOptions {
	o.HeadersComplete = flag
	return o
}

// AgentMatch is the return from findAgent.
type AgentMatch struct {
	Agent   *RawAgentDef // pointer to raw agent definition
	Version string       // version of agent, where applicable
	Reason  string       // reason we matched this agent
	Weak    bool         // is this a weak match?
}

var textBrowserExpr = regexp.MustCompile(`^(Lynx|ELinks|Links|w3m)\/`)
var browserExpr = regexp.MustCompile(`^Mozilla\/[4-6]\.0 \(|^Opera\/\d`)
var browserEngineExpr = regexp.MustCompile(`\(KHTML,? ?like Gecko|KHTML\/[\d.]+ \(like Gecko|Gecko\/\d|\) like Gecko|Trident\/\d|MSIE \d|Presto\/\d|Chromium\/\d`)
var botWordExpr = regexp.MustCompile(`(?:[a-z0-9-]|\b)[Bb]ot(?:\b|\/)`)
var emailExpr = regexp.MustCompile(`(?i)[\w.+-]+@[\w-]+(\.[\w-]+)*\.[a-z]{2,}`)
var automationExpr = regexp.MustCompile(`(?i)crawl|spider|scrap|headless|https?:\/\/|\+http|(?:^|[^a-z])(?:archive|observ|monitor|check(?:er)?\b|preview|verif|render|insight|console|transcod|scanner|synthetic)|lighthouse|selenium|cypress|webdriver|\bsplash\b|gomezagent|rigor|speedcurve|\bptst\b|dareboost|dejaclick|favicon|siteimprove|onetrust|datanyze|feeder\.co|rss\b|weblight|duplexweb|ips-agent`)
var compatibleExpr = regexp.MustCompile(`compatible;`)
var enginesExpr = regexp.MustCompile(`MSIE|Trident|Konqueror`)

// isBrowser retruns true if the user agent indicates a browser.
func isBrowser(ua string) bool {
	if textBrowserExpr.MatchString(ua) {
		return true
	}
	if !browserExpr.MatchString(ua) || !browserEngineExpr.MatchString(ua) {
		return false
	}
	if botWordExpr.MatchString(ua) || emailExpr.MatchString(ua) || automationExpr.MatchString(ua) {
		return false
	}
	return !compatibleExpr.MatchString(ua) || enginesExpr.MatchString(ua)
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

// rankedAcceptData is the type of data returned from rankAccept.
type rankedAcceptData struct {
	mtype string // MIME type
	q1000 int    // quality value multiplied by 1000
	ndx   int    // original index in Accept string
}

// rankAccept turns an Accept string into a list of ranked accept members.
func rankAccept(accept string) []rankedAcceptData {
	components := strings.Split(accept, ",")
	ranks := make([]rankedAcceptData, len(components))
	for i, com := range components {
		subparts := strings.Split(com, ";")
		ranks[i].mtype = subparts[0]
		ranks[i].q1000 = 1000
		ranks[i].ndx = i
		if len(subparts) > 1 {
			for _, part := range subparts[1:] {
				part = strings.TrimSpace(part)
				if strings.HasPrefix(part, "q=") {
					parsed, err := strconv.ParseFloat(part[2:], 64)
					if err != nil {
						ranks[i].q1000 = int(math.Floor(1000.0 * parsed))
					}
					break
				}
			}
		}
	}
	ranks2 := make([]rankedAcceptData, 0, len(ranks))
	for i := range ranks {
		if ranks[i].q1000 > 0 {
			ranks2 = append(ranks2, ranks[i])
		}
	}
	slices.SortFunc(ranks2, func(a, b rankedAcceptData) int {
		t := b.q1000 - a.q1000
		if t == 0 {
			t = b.ndx - a.ndx
		}
		return t
	})
	return ranks2
}

// ranksAboveHtml returns true if any of the given MIME types ranks above HTML in the given Accept: string.
func ranksAboveHtml(accept string, types []string) bool {
	ranked := rankAccept(accept)
	md := slices.IndexFunc(ranked, func(d rankedAcceptData) bool {
		return slices.Contains(types, d.mtype)
	})
	if md < 0 {
		return false
	}
	html := slices.IndexFunc(ranked, func(d rankedAcceptData) bool {
		return d.mtype == "text/html"
	})
	if html < 0 {
		return true
	}
	return md < html
}

// markdownTypes are the MIME types associated with Markdown.
var markdownTypes = []string{"text/markdown", "text/x-markdown"}

// prefersMarkdown returns true if the given Accept: string prefers Markdown over HTML.
func prefersMarkdown(accept string) bool {
	return ranksAboveHtml(accept, markdownTypes)
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

// fromAgent turns an AgentMatch into a Classification.
func fromAgent(match *AgentMatch, rawIP string, opts ClassifyOptions) *Classification {
	agent := match.Agent
	base := new(Classification{
		Category:   agent.Category,
		Kind:       agent.Kind,
		Name:       agent.Name,
		Operator:   agent.Operator,
		Version:    match.Version,
		Verified:   true,
		Confidence: ConfidenceMedium,
		Reasons:    make([]string, 1),
	})
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
	now := opts.Now
	if now == nil {
		tmp := time.Now()
		now = &tmp
	}
	maxDuration := time.Hour * time.Duration(24*opts.MaxAge)
	impersonation := false
	validRangeFiles := false
	for _, url := range urls {
		cidrs, creation, err := getIPRangeData(url)
		if err != nil {
			base.Reasons = append(base.Reasons, fmt.Sprintf("unable to get range file %s: %v", url, err))
			continue
		}
		validRangeFiles = true
		if inRanges(*ip, cidrs) {
			base.Verified = true
			base.Confidence = ConfidenceHigh
			base.Reasons = append(base.Reasons, "%s is in %s's published ranges", *ip, agent.Operator)
			return base
		}
		if now.Sub(*creation) <= maxDuration {
			impersonation = true
		}
	}
	if !validRangeFiles {
		return base
	}
	if impersonation {
		base.Category = "bot"
		base.Kind = "impersonator"
		base.Verified = false
		base.Confidence = ConfidenceHigh
		base.Reasons = append(base.Reasons, fmt.Sprintf("claims to be %s but %s is outside %s's published ranges", agent.Name, *ip, agent.Operator))
		return base
	}
	base.Reasons = append(base.Reasons, fmt.Sprintf("%s's range file is older than %d days, so a mismatch is not counted as spoofing", agent.Operator, opts.MaxAge))
	return base
}

// fake returns a Classification identifying the request as a fake browser.
func fake(reason string, confidence int) *Classification {
	rc := new(Classification{
		Category:   "bot",
		Kind:       "fake-browser",
		Verified:   false,
		Confidence: confidence,
		Reasons:    make([]string, 1),
	})
	rc.Reasons[0] = reason
	return rc
}

// majorVersion extracts a browser's major version from a regular expression.
func majorVersion(ua string, re *regexp.Regexp) int {
	m := re.FindStringSubmatch(ua)
	if m != nil {
		v, err := strconv.Atoi(m[1])
		if err == nil {
			return v
		}
	}
	return 0
}

var firefoxBrowserExpr = regexp.MustCompile(`Firefox\/`)
var firefoxVersionExpr = regexp.MustCompile(`Firefox\/(\d+)`)
var chromeBrowserExpr = regexp.MustCompile(`Chrome\/`)
var chromeVersionExpr = regexp.MustCompile(`Chrome\/(\d+)`)
var safariBrowserExpr = regexp.MustCompile(`Version\/[\d.]+.*Safari\/`)
var safariVersionExpr = regexp.MustCompile(`Version\/(\d+)`)

func sendsFetchMetadata(ua string) bool {
	if firefoxBrowserExpr.MatchString(ua) {
		return majorVersion(ua, firefoxVersionExpr) >= 90
	}
	if chromeBrowserExpr.MatchString(ua) {
		return majorVersion(ua, chromeVersionExpr) >= 80
	}
	if safariBrowserExpr.MatchString(ua) {
		return majorVersion(ua, safariVersionExpr) >= 17
	}
	return false
}

var plainTextType = []string{"text/plain"}

func fromBrowser(ua string, h map[string]string, opts ClassifyOptions) *Classification {
	accept, ok := h["accept"]
	if ok {
		if ranksAboveHtml(accept, plainTextType) {
			return fake("Accept header ranks text/plain above HTML, which browsers never do", ConfidenceLow)
		}
		if strings.Contains(accept, "application/metalink") {
			return fake("Accept header asks for metalink, which download tools send and browsers do not", ConfidenceMedium)
		}
	}
	if !opts.HeadersComplete {
		rc := new(Classification{
			Category:   "human",
			Verified:   false,
			Confidence: ConfidenceLow,
			Reasons:    make([]string, 1),
		})
		rc.Reasons[0] = "browser user agent; headers not checked"
		return rc
	}
	_, ok = h["sec-ch-ua"]
	if ok && !chromeBrowserExpr.MatchString(ua) {
		return fake("sec-ch-ua sent with a non-Chromium user agent", ConfidenceHigh)
	}
	_, ok = h["sec-fetch-mode"]
	if !ok && sendsFetchMetadata(ua) {
		return fake("browser user agent without sec-fetch-* headers", ConfidenceMedium)
	}
	_, ok = h["accept-language"]
	if !ok {
		return fake("browser user agent without Accept-Language", ConfidenceLow)
	}
	rc := new(Classification{
		Category:   "human",
		Verified:   false,
		Confidence: ConfidenceMedium,
		Reasons:    make([]string, 1),
	})
	rc.Reasons[0] = "browser user agent with consistent browser headers"
	return rc
}

func AmClassify(headers http.Header, ipAddr string, userAgent string, opts ...ClassifyOptions) *Classification {
	myopts := AmClassifyOptions()
	if len(opts) > 0 {
		myopts = opts[0]
	}
	h := normalizeHeaders(headers)
	ua := userAgent
	if ua == "" {
		t, ok := h["user-agent"]
		if ok {
			ua = t
		}
	}

	if ua == "" {
		rc := new(Classification{
			Verified:   false,
			Confidence: ConfidenceLow,
			Reasons:    make([]string, 1),
		})
		if len(h) == 0 {
			rc.Category = "unknown"
			rc.Reasons[0] = "no user agent or headers to classify"
		} else {
			rc.Category = "bot"
			rc.Kind = "unidentified"
			rc.Reasons[0] = "request has no user agent, which browsers always send"
		}
		return rc
	}

	// Look for a matching agent.
	match := findAgent(ua, h)
	if match != nil {
		return fromAgent(match, ipAddr, myopts)
	}

	// Check the Accept: line; if the request prefers Markdown to HTML, we're some kind of agent.
	acc, ok := h["accept"]
	if ok {
		if prefersMarkdown(acc) {
			rc := new(Classification{
				Category:   "agent",
				Kind:       "unidentified",
				Verified:   false,
				Confidence: ConfidenceMedium,
				Reasons:    make([]string, 1),
			})
			rc.Reasons[0] = "Accept header prefers markdown over HTML, which browsers never do"
			return rc
		}
	}

	// Look for an HTTP client.
	ndx := slices.IndexFunc(compiledClients, func(d clientDef) bool {
		return d.pattern.MatchString(ua)
	})
	if ndx >= 0 {
		rc := new(Classification{
			Category:   "bot",
			Kind:       compiledClients[ndx].r.Kind,
			Name:       compiledClients[ndx].r.Name,
			Verified:   false,
			Confidence: ConfidenceMedium,
			Reasons:    make([]string, 1),
		})
		rc.Reasons[0] = fmt.Sprintf("user agent is %s", compiledClients[ndx].r.Name)
		return rc
	}
	if isBrowser(ua) {
		return fromBrowser(ua, h, myopts)
	}
	rc := new(Classification{
		Category:   "bot",
		Kind:       "unidentified",
		Verified:   false,
		Confidence: ConfidenceLow,
		Reasons:    make([]string, 1),
	})
	rc.Reasons[0] = "user agent is not a browser and matches no known agent, crawler or client"
	return rc
}

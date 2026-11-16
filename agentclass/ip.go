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
	"net/netip"
	"regexp"
	"slices"
	"strings"
)

// parseIP turns a string IP address into a ParsedIP. Returns nil if address is not parseable.
func parseIP(ip string) *netip.Addr {
	s := strings.TrimSpace(ip)
	v6 := strings.Contains(s, ":")
	addy, err := netip.ParseAddr(s)
	if err != nil || addy.Is6() != v6 {
		return nil
	}
	return &addy
}

// bracketedExpr recognizes an IP address with brackets.
var bracketedExpr = regexp.MustCompile(`^\[([^\]]+)\](?::\d+)?$`)

// v4WithPortExpr recognizes an IPv4 address with a port.
var v4WithPortExpr = regexp.MustCompile(`^(\d{1,3}(?:\.\d{1,3}){3}):\d+$`)

// normalizeIP filters a supplied IP address. Returns nil if the address is not parseable.
func normalizeIP(ip string) *string {
	s := strings.TrimSpace(ip)
	if s == "" || strings.Contains(s, ",") {
		return nil
	}
	bracketed := bracketedExpr.FindStringSubmatch(s)
	v4WithPort := v4WithPortExpr.FindStringSubmatch(s)
	candidate := s
	if bracketed != nil {
		candidate = bracketed[1]
	} else if v4WithPort != nil {
		candidate = v4WithPort[1]
	}
	parsed := parseIP(candidate)
	if parsed == nil {
		return nil
	}
	if parsed.Is4In6() {
		return new(netip.AddrFrom4(parsed.As4()).String())
	}
	return new(candidate)
}

func inRanges(ip string, cidrs []netip.Prefix) bool {
	normalized := normalizeIP(ip)
	if normalized == nil {
		return false
	}
	addy := parseIP(*normalized)
	return slices.IndexFunc(cidrs, func(c netip.Prefix) bool {
		return c.Contains(*addy)
	}) >= 0
}

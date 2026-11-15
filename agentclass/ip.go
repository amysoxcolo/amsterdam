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
	"net/netip"
	"regexp"
	"strings"
)

// ParsedIP is the structure that holds a parsed IP address.
type ParsedIP struct {
	V6    bool       // is this a v6 address?
	Value netip.Addr // the actual address
}

// parseIP turns a string IP address into a ParsedIP. Returns nil if address is not parseable.
func parseIP(ip string) *ParsedIP {
	s := strings.TrimSpace(ip)
	v6 := strings.Index(s, ":") >= 0
	addy, err := netip.ParseAddr(s)
	if err != nil || addy.Is6() != v6 {
		return nil
	}
	return new(ParsedIP{V6: v6, Value: addy})
}

// bracketedExpr recognizes an IP address with brackets.
var bracketedExpr = regexp.MustCompile(`^\[([^\]]+)\](?::\d+)?$`)

// v4WithPortExpr recognizes an IPv4 address with a port.
var v4WithPortExpr = regexp.MustCompile(`^(\d{1,3}(?:\.\d{1,3}){3}):\d+$`)

// normalizeIP filters a supplied IP address. Returns nil if the address is not parseable.
func normalizeIP(ip string) *string {
	s := strings.TrimSpace(ip)
	if s == "" || strings.Index(s, ",") >= 0 {
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
	if parsed.V6 && parsed.Value.Is4In6() {
		return new(netip.AddrFrom4(parsed.Value.As4()).String())
	}
	return new(candidate)
}

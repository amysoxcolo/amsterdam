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
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"net/http"
	"sync"
	"time"
)

const NUM_RETRIES = 3
const TIMEOUT = 10

type RangeDataPrefix struct {
	IPv4Prefix string `json:"ipv4Prefix"`
	IPv6Prefix string `json:"ipv6Prefix"`
}

type RangeDataFormat struct {
	CreationTime *time.Time        `json:"creationTime"`
	Prefixes     []RangeDataPrefix `json:"prefixes"`
}

type ipRangeData struct {
	m         sync.Mutex
	c         *sync.Cond
	loaded    bool
	loadError error
	creation  *time.Time
	cidrs     []string
}

var ipRangeStore map[string]*ipRangeData

func loadIPRangeData(ctx context.Context, url string, rd *ipRangeData) {
	rd.m.Lock()
	defer rd.m.Unlock()
	for counter := NUM_RETRIES; counter > 0; counter-- {
		select {
		case <-ctx.Done():
			rd.loadError = ctx.Err()
			break
		default:
			client := &http.Client{Timeout: TIMEOUT * time.Second}
			resp, err := client.Get(url)
			select {
			case <-ctx.Done():
				resp.Body.Close()
				rd.loadError = ctx.Err()
				break
			default:
				if err == nil {
					defer resp.Body.Close()
					if resp.StatusCode == http.StatusOK {
						decoder := jsontext.NewDecoder(resp.Body)
						var parsed RangeDataFormat
						err = json.UnmarshalDecode(decoder, &parsed)
						if err == nil {
							lx := 0
							if parsed.Prefixes != nil {
								lx = len(parsed.Prefixes)
							}

						} else {
							rd.loadError = err
						}
					} else {
						rd.loadError = fmt.Errorf("failed to get file with status %d", resp.StatusCode)
					}
				} else {
					rd.loadError = err
				}
			}
		}
	}
	rd.loaded = true
	rd.c.Signal()
}

func loadIPRanges(urls []string, ctxname any) {
	ipRangeStore = make(map[string]*ipRangeData)
	ctx, cancelfunc := context.WithCancel(context.Background())
	ctx = context.WithValue(ctx, ctxname, "loadIPRanges")
	for _, url := range urls {
		rdat := new(ipRangeData{loaded: false, loadError: nil, creation: nil})
		rdat.c = sync.NewCond(&(rdat.m))
	}
}

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
	"net/netip"
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
	cidrs     []netip.Prefix
}

var ipRangeStore map[string]*ipRangeData

func loadIPRangeData(ctx context.Context, url string, rd *ipRangeData) {
	rd.m.Lock()
	defer rd.m.Unlock()
LOADLOOP:
	for counter := NUM_RETRIES; counter > 0; counter-- {
		select {
		case <-ctx.Done():
			rd.loadError = ctx.Err()
			break LOADLOOP
		default:
			client := &http.Client{Timeout: TIMEOUT * time.Second}
			resp, err := client.Get(url)
			select {
			case <-ctx.Done():
				resp.Body.Close()
				rd.loadError = ctx.Err()
				break LOADLOOP
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
							rd.cidrs = make([]netip.Prefix, lx)
							if lx > 0 {
								for i, p := range parsed.Prefixes {
									if p.IPv4Prefix == "" {
										rd.cidrs[i], err = netip.ParsePrefix(p.IPv6Prefix)
									} else {
										rd.cidrs[i], err = netip.ParsePrefix(p.IPv4Prefix)
									}
									if err != nil {
										rd.loadError = err
										break LOADLOOP
									}
								}
							}
							rd.loadError = nil
							break LOADLOOP
						} else {
							rd.loadError = err
							break LOADLOOP
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
	select {
	case <-ctx.Done():
		if rd.loadError == nil {
			rd.loadError = ctx.Err()
		}
	default:
		// fall through
	}
	rd.loaded = true
	rd.c.Signal()
}

var loadRangesCtx context.Context

func loadIPRanges(urls []string, ctxname any) func() {
	ipRangeStore = make(map[string]*ipRangeData)
	ctx, cancelfunc := context.WithCancel(context.Background())
	loadRangesCtx = context.WithValue(ctx, ctxname, "loadIPRanges")
	for _, url := range urls {
		rdat := new(ipRangeData{loaded: false, loadError: nil, creation: nil})
		rdat.c = sync.NewCond(&(rdat.m))
		ipRangeStore[url] = rdat
		go loadIPRangeData(loadRangesCtx, url, rdat)
	}
	return func() {
		cancelfunc()
		for _, v := range ipRangeStore {
			v.m.Lock()
			if !v.loaded {
				v.loadError = context.Canceled
				v.c.Signal()
			}
			v.m.Unlock()
		}
	}
}

func getIPRangeData(url string) ([]netip.Prefix, *time.Time, error) {
	rdat, ok := ipRangeStore[url]
	if !ok {
		rdat := new(ipRangeData{loaded: false, loadError: nil, creation: nil})
		rdat.c = sync.NewCond(&(rdat.m))
		ipRangeStore[url] = rdat
		go loadIPRangeData(loadRangesCtx, url, rdat)
	}
	rdat.m.Lock()
	defer rdat.m.Unlock()
	for !rdat.loaded {
		rdat.c.Wait()
	}
	return rdat.cidrs, rdat.creation, rdat.loadError
}

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
	"maps"
	"net/http"
	"net/netip"
	"sync"
	"time"

	"git.erbosoft.com/amy/amsterdam/config"
	"github.com/labstack/gommon/log"
)

// Format for IP range data inside the RangeDataFormat.
type RangeDataPrefix struct {
	IPv4Prefix string `json:"ipv4Prefix"` // prefix, in IPv4 format
	IPv6Prefix string `json:"ipv6Prefix"` // prefix, in IPv6 format
}

// Format of IP range data files exported by bot providers.
type RangeDataFormat struct {
	CreationTime *time.Time        `json:"creationTime"` // creation date and time
	Prefixes     []RangeDataPrefix `json:"prefixes"`     // the prefixes
}

// Stored IP range data for a single URL.
type ipRangeData struct {
	m         sync.Mutex     // mutex on the range data
	c         *sync.Cond     // signals when load complete
	loaded    bool           // set when load operation completed
	loadError error          // error during load, or nil
	lastMod   string         // Last-Modified header from last fetch
	creation  *time.Time     // creation date and time
	cidrs     []netip.Prefix // the prefixes from this file
}

// Contains all the data for loaded IP range files.
var ipRangeStore map[string]*ipRangeData

// loadIPRangeData is the goroutine that fetches range data from a URL and parses it.
func loadIPRangeData(ctx context.Context, url string, rd *ipRangeData) {
	rd.m.Lock()
	defer rd.m.Unlock()
LOADLOOP:
	for counter := config.GlobalConfig.Tuning.IPRangeLoadAttempts; counter > 0; counter-- {
		select {
		case <-ctx.Done():
			rd.loadError = ctx.Err()
			break LOADLOOP
		default:
			req, err := http.NewRequest("GET", url, nil)
			if err != nil {
				rd.loadError = err
				break LOADLOOP
			}
			if rd.lastMod != "" {
				req.Header.Set("If-Modified-Since", rd.lastMod)
			}
			client := &http.Client{Timeout: time.Duration(config.GlobalConfig.Tuning.Timeouts.IPRangeLoad) * time.Second}
			resp, err := client.Do(req)
			select {
			case <-ctx.Done():
				resp.Body.Close()
				rd.loadError = ctx.Err()
				break LOADLOOP
			default:
				if err != nil {
					rd.loadError = err
					continue LOADLOOP
				}
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusNotModified {
					rd.loadError = nil
					break LOADLOOP
				}
				if resp.StatusCode != http.StatusOK {
					rd.loadError = fmt.Errorf("failed to get file with status %d", resp.StatusCode)
					continue LOADLOOP
				}
				lastmod := ""
				if lm, ok := resp.Header["Last-Modified"]; ok {
					lastmod = lm[0]
				}
				decoder := jsontext.NewDecoder(resp.Body)
				var parsed RangeDataFormat
				err = json.UnmarshalDecode(decoder, &parsed)
				if err != nil {
					rd.loadError = err
					break LOADLOOP
				}
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
				rd.lastMod = lastmod
				break LOADLOOP
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

func prepareRefresh(rd *ipRangeData) {
	rd.m.Lock()
	defer rd.m.Unlock()
	for !rd.loaded {
		rd.c.Wait()
	}
	rd.loaded = false
}

func refreshRanges(ctx context.Context, interval time.Duration) {
	tick := time.NewTicker(interval)
REFRESHLOOP:
	for {
		select {
		case <-ctx.Done():
			tick.Stop()
			break REFRESHLOOP
		case <-tick.C:
			mymap := make(map[string]*ipRangeData)
			maps.Copy(mymap, ipRangeStore)
			for url, rd := range mymap {
				select {
				case <-ctx.Done():
					tick.Stop()
					break REFRESHLOOP
				default:
					prepareRefresh(rd)
					go loadIPRangeData(ctx, url, rd)
				}
			}
		}
	}
}

var loadRangesCtx context.Context = nil

func loadIPRanges(urls []string, ctxname any) func() {
	ipRangeStore = make(map[string]*ipRangeData)
	ctx, cancelfunc := context.WithCancel(context.Background())
	loadRangesCtx = context.WithValue(ctx, ctxname, "loadIPRanges")
	for _, url := range urls {
		rdat := new(ipRangeData{loaded: false, loadError: nil, lastMod: "", creation: nil})
		rdat.c = sync.NewCond(&(rdat.m))
		ipRangeStore[url] = rdat
		go loadIPRangeData(loadRangesCtx, url, rdat)
	}
	dur, err := time.ParseDuration(config.GlobalConfig.Tuning.IPRangeRefresh)
	if err != nil {
		log.Errorf("invalid IP range refresh duration \"%s\", using default", config.GlobalConfig.Tuning.IPRangeRefresh)
		dur = 24 * time.Hour
	}
	go refreshRanges(loadRangesCtx, dur)
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
		rdat := new(ipRangeData{loaded: false, loadError: nil, lastMod: "", creation: nil})
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

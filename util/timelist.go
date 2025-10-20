/*
 * Amsterdam Web Communities System
 * Copyright (c) 2025 Erbosoft Metaverse Design Solutions, All Rights Reserved
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

// Package util contains utility definitions.
package util

import (
	"slices"
	"sync"
	"time"

	"github.com/tkuchiki/go-timezone"
)

// cachedTimeZoneList is a wrapper around timezone.Timezones() that produces it by timezone name.
var cachedTimeZoneList []string = nil

// timeZoneListMutex controls access to internalGetTimeZoneList.
var timeZoneListMutex sync.Mutex

// AmTimeZoneList is a wrapper around TimeZone.TimeZones() that sorts and compacts the list.
func AmTimeZoneList() []string {
	timeZoneListMutex.Lock()
	defer timeZoneListMutex.Unlock()
	if cachedTimeZoneList == nil {
		timezones := timezone.New().Timezones()
		ilist := make([]string, 0, len(timezones)*5)
		for k, v := range timezones {
			ilist = append(ilist, k)
			ilist = append(ilist, v...)
		}

		slices.Sort(ilist)
		cachedTimeZoneList = slices.Compact(ilist)
	}
	return cachedTimeZoneList
}

// AmMonthList is a simple wrapper that returns the names of the months to the template context.
func AmMonthList() []string {
	rc := make([]string, 12)
	for m := time.January; m <= time.December; m++ {
		rc[m-time.January] = m.String()
	}
	return rc
}

// init preloads the time zone list.
func init() {
	go AmTimeZoneList()
}

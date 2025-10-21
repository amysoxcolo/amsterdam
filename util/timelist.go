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
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/klauspost/lctime"
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

/* AmActivityString generates a string to represent the activity based on the given timestamp
 * and the current time.
 * Parameters:
 *     timeval - The time value representing the last point of activity.
 *     loc - The localizer used to format the time.
 * Returns:
 *     The string activity equivalent.
 */
func AmActivityString(timeval *time.Time, loc lctime.Localizer) string {
	if timeval == nil {
		return "Never"
	}
	now := time.Now().In(timeval.Location())
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if timeval.Compare(day) == 1 {
		return "Today, " + loc.Strftime("%X", *timeval)
	}
	day = time.Date(now.Year(), now.Month(), now.Day()-1, 0, 0, 0, 0, now.Location())
	if timeval.Compare(day) == 1 {
		return "Yesterday, " + loc.Strftime("%X", *timeval)
	}
	duration := now.Sub(*timeval)
	days := duration.Hours() / 24.0
	day = time.Date(now.Year(), now.Month()-1, now.Day(), 0, 0, 0, 0, now.Location())
	if timeval.Compare(day) == 1 {
		return fmt.Sprintf("%d days ago", int(days))
	}
	day = time.Date(now.Year()-1, now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if timeval.Compare(day) == 1 {
		nm := int(days / 30.0)
		if nm == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", nm)
	}
	ny := int(days / 365.25)
	if ny == 1 {
		return "1 year ago"
	}
	return fmt.Sprintf("%d years ago", ny)
}

// init preloads the time zone list.
func init() {
	go AmTimeZoneList()
}

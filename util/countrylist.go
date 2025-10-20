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
	"strings"
	"sync"

	"git.erbosoft.com/amy/amsterdam/config"
	"github.com/biter777/countries"
)

// cachedCountryList is the cached country list after sorting.
var cachedCountryList []countries.CountryCode = nil

// countryListMutex control access to internalGetCountryList.
var countryListMutex sync.Mutex

// AmCountryList is a wrapper around countries.All() that sorts it by country name.
func AmCountryList() []countries.CountryCode {
	countryListMutex.Lock()
	defer countryListMutex.Unlock()
	if cachedCountryList == nil {
		countryList := countries.All()
		slices.SortFunc(countryList, func(a countries.CountryCode, b countries.CountryCode) int {
			return strings.Compare(a.Info().Name, b.Info().Name)
		})
		if config.GlobalConfig.Rendering.CountryList.Prioritize != "" {
			for i, c := range countryList {
				if c.Info().Alpha2 == config.GlobalConfig.Rendering.CountryList.Prioritize {
					newList := make([]countries.CountryCode, len(countryList))
					newList[0] = c
					copy(newList[1:], countryList[:i])
					copy(newList[i+1:], countryList[i+1:])
					countryList = newList
				}
			}
		}
		cachedCountryList = countryList
	}
	return cachedCountryList
}

// init preloads the country list.
func init() {
	go AmCountryList()
}

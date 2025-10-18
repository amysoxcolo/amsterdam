/*
 * Amsterdam Web Communities System
 * Copyright (c) 2025 Erbosoft Metaverse Design Solutions, All Rights Reserved
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

// Package ui holds the support for the Amsterdam user interface, wrapping Echo and Jet templates.
package ui

import (
	"embed"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"git.erbosoft.com/amy/amsterdam/config"
	"git.erbosoft.com/amy/amsterdam/database"
	"git.erbosoft.com/amy/amsterdam/util"
	"github.com/CloudyKit/jet/v6"
	"github.com/CloudyKit/jet/v6/loaders/embedfs"
	"github.com/CloudyKit/jet/v6/loaders/multi"
	"github.com/biter777/countries"
	"github.com/labstack/echo/v4"
	log "github.com/sirupsen/logrus"
	"github.com/tkuchiki/go-timezone"
	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
)

//go:embed views/*
var static_views embed.FS

// views is the main Jet template repository.
var views *jet.Set

//go:embed languages.txt
var knownLanguages string

// Language is a type for a list of all supportred languages.
type Language struct {
	Tag  string // the BCP 47 tag, such as "en-US"
	Name string // the human-readable name, like "American English"
}

// cachedLanguageList is the cached language list.
var cachedLanguageList []Language = nil

// languageListMutex controls access to internalGetLanguageList.
var languageListMutex sync.Mutex

// internalGetLanguageList is a wrapper around "allTags" that sorts it by language name.
func internalGetLanguageList() []Language {
	languageListMutex.Lock()
	defer languageListMutex.Unlock()
	if cachedLanguageList == nil {
		langs := strings.Split(knownLanguages, "\n")
		enNamer := display.English.Tags()
		cachedLanguageList = make([]Language, 0, len(langs))
		for _, l := range langs {
			tag, err := language.Parse(l)
			if err == nil {
				cachedLanguageList = append(cachedLanguageList, Language{
					Tag:  tag.String(),
					Name: enNamer.Name(tag),
				})
			} else {
				log.Errorf("*** PUKE on parsing language tag %s: %v", l, err)
			}
		}

		slices.SortFunc(cachedLanguageList, func(a Language, b Language) int {
			return strings.Compare(a.Name, b.Name)
		})
	}
	return cachedLanguageList
}

// cachedTimeZoneList is a wrapper around timezone.Timezones() that produces it by timezone name.
var cachedTimeZoneList []string = nil

// timeZoneListMutex controls access to internalGetTimeZoneList.
var timeZoneListMutex sync.Mutex

// internalGetTimeZoneList is a wrapper around TimeZone.TimeZones() that sorts and compacts the list.
func internalGetTimeZoneList() []string {
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

// cachedCountryList is the cached country list after sorting.
var cachedCountryList []countries.CountryCode = nil

// countryListMutex control access to internalGetCountryList.
var countryListMutex sync.Mutex

// internalGetCountryList is a wrapper around countries.All() that sorts it by country name.
func internalGetCountryList() []countries.CountryCode {
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

// getMonthList is a simple wrapper that returns the names of the months to the template context.
func getMonthList(a jet.Arguments) reflect.Value {
	rc := make([]string, 12)
	for m := time.January; m <= time.December; m++ {
		rc[m-time.January] = m.String()
	}
	return reflect.ValueOf(rc)
}

// countRanger is a Ranger that can count from one value to another with a certain step.
type countRanger struct {
	i    int
	val  int64
	step int64
	to   int64
}

/* Range (from Ranger) returns the "next" value of this iterator.
 * Returns:
 *     Next index of the returned value
 *     Next returned value
 *     true if this is the last iteration, false if not
 */
func (r *countRanger) Range() (reflect.Value, reflect.Value, bool) {
	r.i++
	r.val += r.step
	var end bool
	if r.step < 0 {
		end = r.val <= r.to
	} else {
		end = r.val >= r.to
	}
	return reflect.ValueOf(r.i), reflect.ValueOf(r.val), end
}

// ProvidesIndex (from Ranger) returns true to indicate that this Ranger has indexes.
func (r *countRanger) ProvidesIndex() bool {
	return true
}

// makeIntRange creates and returns a countRanger.
func makeIntRange(a jet.Arguments) reflect.Value {
	from := a.Get(0).Convert(reflect.TypeFor[int64]()).Int()
	to := a.Get(1).Convert(reflect.TypeFor[int64]()).Int()
	step := a.Get(2).Convert(reflect.TypeFor[int64]()).Int()
	rc := &countRanger{i: -1, val: from - step, step: step, to: to}
	return reflect.ValueOf(rc).Convert(reflect.TypeFor[jet.Ranger]())
}

// makeYearRange parses a year parameter and creates a countRanger that reflects it.
func makeYearRange(a jet.Arguments) reflect.Value {
	param := a.Get(0).Convert(reflect.TypeFor[string]()).String()
	yearRegex, _ := regexp.Compile(`year:(\S+)(\s+.+)?$`)
	m := yearRegex.FindStringSubmatch(param)
	if m != nil {
		count, err := strconv.Atoi(m[1])
		if err == nil {
			start_year := time.Now().Year()
			rc := &countRanger{i: -1, val: int64(start_year) + 1, step: -1, to: int64(start_year + count - 1)}
			return reflect.ValueOf(rc).Convert(reflect.TypeFor[jet.Ranger]())
		} else {
			return reflect.ValueOf(err)
		}
	} else {
		return reflect.ValueOf(fmt.Errorf("cannot locate year: marker in param"))
	}
}

// SetupTemplates is called to set up the template renderer after the configuration is loaded.
func SetupTemplates() {
	views = jet.NewSet(
		multi.NewLoader(
			jet.NewOSFileSystemLoader(config.GlobalConfig.Rendering.TemplateDir),
			embedfs.NewLoader("views/", static_views),
		),
		jet.DevelopmentMode(true),
	)
	views.AddGlobal("AmsterdamVersion", config.AMSTERDAM_VERSION)
	views.AddGlobal("AmsterdamCopyright", config.AMSTERDAM_COPYRIGHT)
	views.AddGlobal("GlobalConfig", config.GlobalConfig)
	views.AddGlobalFunc("GetMonthList", getMonthList)
	views.AddGlobalFunc("MakeIntRange", makeIntRange)
	views.AddGlobalFunc("MakeYearRange", makeYearRange)

	views.AddGlobalFunc("GetCountryList", func(a jet.Arguments) reflect.Value {
		return reflect.ValueOf(internalGetCountryList())
	})
	views.AddGlobalFunc("GetLanguageList", func(a jet.Arguments) reflect.Value {
		return reflect.ValueOf(internalGetLanguageList())
	})
	views.AddGlobalFunc("GetTimeZoneList", func(a jet.Arguments) reflect.Value {
		return reflect.ValueOf(internalGetTimeZoneList())
	})
	views.AddGlobalFunc("AmMenu", func(a jet.Arguments) reflect.Value {
		s := a.Get(0).Convert(reflect.TypeFor[string]()).String()
		return reflect.ValueOf(AmMenu(s))
	})
	views.AddGlobalFunc("AmRoleList", func(a jet.Arguments) reflect.Value {
		s := a.Get(0).Convert(reflect.TypeFor[string]()).String()
		return reflect.ValueOf(database.AmRoleList(s))
	})
	views.AddGlobalFunc("CapitalizeString", func(a jet.Arguments) reflect.Value {
		s := a.Get(0).Convert(reflect.TypeFor[string]()).String()
		return reflect.ValueOf(util.CapitalizeString(s))
	})

	// preload the lists in the background
	go internalGetCountryList()
	go internalGetLanguageList()
	go internalGetTimeZoneList()
}

// TemplateRenderer is the Renderer instance set into the Echo context at creation time, to render Jet templates.
type TemplateRenderer struct{}

/* Render renders a Jet template to the Echo output stream.
 * Parameters:
 *     w - Echo's output stream writer.
 *     name - Name of the template to be rendered.
 *     data - Context data to pass to the template.
 *     c - The Echo context for the request being processed.
 * Returns:
 *     Standard Go error status.
 */
func (r *TemplateRenderer) Render(w io.Writer, name string, data any, c echo.Context) error {
	view, err := views.GetTemplate(name)

	if err != nil {
		log.Errorf("Unable to load template \"%s\": %v", name, err)
		return err
	}
	var vmap jet.VarMap = nil
	amctxt := AmContextFromEchoContext(c)
	if amctxt != nil {
		vmap = amctxt.VarMap()
	}
	return view.Execute(w, vmap, data)
}

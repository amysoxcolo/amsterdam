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
	"strconv"
	"time"

	"git.erbosoft.com/amy/amsterdam/config"
	"git.erbosoft.com/amy/amsterdam/database"
	"git.erbosoft.com/amy/amsterdam/util"
	"github.com/CloudyKit/jet/v6"
	"github.com/CloudyKit/jet/v6/loaders/embedfs"
	"github.com/CloudyKit/jet/v6/loaders/multi"
	"github.com/labstack/echo/v4"
	log "github.com/sirupsen/logrus"
)

//go:embed views/*
var static_views embed.FS

// views is the main Jet template repository.
var views *jet.Set

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

// extractCommunityLogo extracts a community logo URL from a community.
func extractCommunityLogo(a jet.Arguments) reflect.Value {
	rc := "/img/builtin/default-community.jpg"
	comm := a.Get(0).Convert(reflect.TypeFor[*database.Community]()).Interface().(*database.Community)
	ci, err := comm.ContactInfo()
	if err == nil {
		if ci.PhotoURL != nil && *ci.PhotoURL != "" {
			rc = *ci.PhotoURL
		}
	}
	return reflect.ValueOf(rc)
}

func displayActivity(a jet.Arguments) reflect.Value {
	timeval := a.Get(0).Convert(reflect.TypeFor[*time.Time]()).Interface().(*time.Time)
	ctxt := a.Get(1).Convert(reflect.TypeFor[AmContext]()).Interface().(AmContext)
	prefs, err := ctxt.CurrentUser().Prefs()
	if err == nil {
		return reflect.ValueOf(util.AmActivityString(timeval, prefs.Localizer()))
	}
	return reflect.ValueOf(fmt.Sprintf("<<%v>>", err))
}

func displayMemberCount(a jet.Arguments) reflect.Value {
	showHidden := false
	comm := a.Get(0).Convert(reflect.TypeFor[*database.Community]()).Interface().(*database.Community)
	ctxt := a.Get(1).Convert(reflect.TypeFor[AmContext]()).Interface().(AmContext)
	level := ctxt.CurrentUser().BaseLevel
	mbr, _, clevel, err := comm.Membership(ctxt.CurrentUser())
	if err == nil {
		if mbr && clevel > level {
			level = clevel
		}
		showHidden = comm.TestPermission("Community.ShowHiddenMembers", level)
	}
	count, err := comm.MemberCount(showHidden)
	if err != nil {
		return reflect.ValueOf(-1)
	}
	return reflect.ValueOf(count)
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
	views.AddGlobalFunc("MakeIntRange", makeIntRange)
	views.AddGlobalFunc("MakeYearRange", makeYearRange)
	views.AddGlobalFunc("ExtractCommunityLogo", extractCommunityLogo)
	views.AddGlobalFunc("DisplayActivity", displayActivity)
	views.AddGlobalFunc("DisplayMemberCount", displayMemberCount)

	views.AddGlobalFunc("GetCountryList", func(jet.Arguments) reflect.Value {
		return reflect.ValueOf(util.AmCountryList())
	})
	views.AddGlobalFunc("GetLanguageList", func(jet.Arguments) reflect.Value {
		return reflect.ValueOf(util.AmLanguageList())
	})
	views.AddGlobalFunc("GetTimeZoneList", func(jet.Arguments) reflect.Value {
		return reflect.ValueOf(util.AmTimeZoneList())
	})
	views.AddGlobalFunc("GetMonthList", func(jet.Arguments) reflect.Value {
		return reflect.ValueOf(util.AmMonthList())
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
	err = view.Execute(w, vmap, data)
	if err != nil {
		log.Errorf("Template \"%s\" failed exec: %v", name, err)
	}
	return err
}

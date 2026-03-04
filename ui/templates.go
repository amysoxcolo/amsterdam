/*
 * Amsterdam Web Communities System
 * Copyright (c) 2025-2026 Erbosoft Metaverse Design Solutions, All Rights Reserved
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

// Package ui holds the support for the Amsterdam user interface, wrapping Echo and Jet templates.
package ui

import (
	"embed"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"git.erbosoft.com/amy/amsterdam/config"
	"git.erbosoft.com/amy/amsterdam/database"
	"git.erbosoft.com/amy/amsterdam/htmlcheck"
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
		return reflect.ValueOf(errors.New("cannot locate year: marker in param"))
	}
}

// immediateIf returns its 2nd argument if its 1st argument is true, and its 3rd argument if its 1st argument is false.
// (Like "a ? b : c" in C or Java.)
func immediateIf(a jet.Arguments) reflect.Value {
	cond := a.Get(0).Convert(reflect.TypeFor[bool]()).Bool()
	if cond {
		return a.Get(1)
	} else {
		return a.Get(2)
	}
}

// extractCommunityLogo extracts a community logo URL from a community.
func extractCommunityLogo(a jet.Arguments) reflect.Value {
	rc := "/img/builtin/default-community.jpg"
	comm := a.Get(0).Convert(reflect.TypeFor[*database.Community]()).Interface().(*database.Community)
	ctxt := a.Get(1).Convert(reflect.TypeFor[AmContext]()).Interface().(AmContext)
	ci, err := comm.ContactInfo(ctxt.Ctx())
	if err == nil {
		if ci.PhotoURL != nil && *ci.PhotoURL != "" {
			rc = *ci.PhotoURL
		}
	}
	return reflect.ValueOf(rc)
}

// communityHost retrieves the community host for a community.
func communityHost(a jet.Arguments) reflect.Value {
	comm := a.Get(0).Convert(reflect.TypeFor[*database.Community]()).Interface().(*database.Community)
	ctxt := a.Get(1).Convert(reflect.TypeFor[AmContext]()).Interface().(AmContext)
	u, err := comm.Host(ctxt.Ctx())
	if err != nil {
		u, _ = database.AmGetAnonUser(ctxt.Ctx())
	}
	return reflect.ValueOf(u)
}

// userContactInfo retrieves the contact info for a user.
func userContactInfo(a jet.Arguments) reflect.Value {
	user := a.Get(0).Convert(reflect.TypeFor[*database.User]()).Interface().(*database.User)
	ctxt := a.Get(1).Convert(reflect.TypeFor[AmContext]()).Interface().(AmContext)
	ci, err := user.ContactInfo(ctxt.Ctx())
	if err != nil {
		ci = database.AmNewUserContactInfo(0)
	}
	return reflect.ValueOf(ci)
}

// displayDateTime formats a date and time value.
func displayDateTime(a jet.Arguments) reflect.Value {
	var timeval time.Time
	p0 := a.Get(0)
	if p0.CanConvert(reflect.TypeFor[time.Time]()) {
		timeval = p0.Convert(reflect.TypeFor[time.Time]()).Interface().(time.Time)
	} else if p0.CanConvert(reflect.TypeFor[*time.Time]()) {
		ptr := p0.Convert(reflect.TypeFor[*time.Time]()).Interface().(*time.Time)
		if ptr == nil {
			return reflect.ValueOf("<<NIL>>")
		}
		timeval = *ptr
	} else {
		return reflect.ValueOf("<<BOGUS>>")
	}
	ctxt := a.Get(1).Convert(reflect.TypeFor[AmContext]()).Interface().(AmContext)
	prefs, err := ctxt.CurrentUser().Prefs(ctxt.Ctx())
	if err == nil {
		loc := prefs.Localizer()
		return reflect.ValueOf(loc.Strftime("%b %e, %Y %r", timeval))
	}
	return reflect.ValueOf(fmt.Sprintf("<<%v>>", err))
}

// displayActivity displays an activity string formatted to the user's preferences.
func displayActivity(a jet.Arguments) reflect.Value {
	timeval := a.Get(0).Convert(reflect.TypeFor[*time.Time]()).Interface().(*time.Time)
	ctxt := a.Get(1).Convert(reflect.TypeFor[AmContext]()).Interface().(AmContext)
	prefs, err := ctxt.CurrentUser().Prefs(ctxt.Ctx())
	if err == nil {
		return reflect.ValueOf(util.AmActivityString(timeval, prefs.Localizer()))
	}
	return reflect.ValueOf(fmt.Sprintf("<<%v>>", err))
}

// displayMemberCount displays the current member count of a community.
func displayMemberCount(a jet.Arguments) reflect.Value {
	showHidden := false
	comm := a.Get(0).Convert(reflect.TypeFor[*database.Community]()).Interface().(*database.Community)
	ctxt := a.Get(1).Convert(reflect.TypeFor[AmContext]()).Interface().(AmContext)
	level := ctxt.CurrentUser().BaseLevel
	mbr, _, clevel, err := comm.Membership(ctxt.Ctx(), ctxt.CurrentUser())
	if err == nil {
		if mbr && clevel > level {
			level = clevel
		}
		showHidden = comm.TestPermission("Community.ShowHiddenMembers", level)
	}
	count, err := comm.MemberCount(ctxt.Ctx(), showHidden)
	if err != nil {
		return reflect.ValueOf(-1)
	}
	return reflect.ValueOf(count)
}

// displayFullName extracts a full name from a contact record.
func displayFullName(a jet.Arguments) reflect.Value {
	ci := a.Get(0).Convert(reflect.TypeFor[*database.ContactInfo]()).Interface().(*database.ContactInfo)
	return reflect.ValueOf(ci.FullName(true))
}

// displayExpandCat displays a category expanded into a hierarchy.
func displayExpandCat(a jet.Arguments) reflect.Value {
	cat := a.Get(0).Convert(reflect.TypeFor[*database.Category]()).Interface().(*database.Category)
	ctxt := a.Get(1).Convert(reflect.TypeFor[AmContext]()).Interface().(AmContext)
	hier, _ := database.AmGetCategoryHierarchy(ctxt.Ctx(), cat.CatId)
	var rc strings.Builder
	for i, c := range hier {
		if i > 0 {
			rc.WriteString(": ")
		}
		rc.WriteString(c.Name)
	}
	return reflect.ValueOf(rc.String())
}

// postRewrite rewrites "x-postlink:" and "x-userlink:" URLs in its text.
func postRewrite(a jet.Arguments) reflect.Value {
	data := a.Get(0).Convert(reflect.TypeFor[string]()).String()
	plIndex := strings.Index(data, htmlcheck.PostLinkURLPrefix)
	ulIndex := strings.Index(data, htmlcheck.UserLinkURIPRefix)
	if plIndex < 0 && ulIndex < 0 {
		return reflect.ValueOf(data)
	}

	if plIndex >= 0 {
		var buf strings.Builder
		t := data
		for plIndex >= 0 {
			if plIndex > 0 {
				buf.WriteString(t[:plIndex])
				t = t[plIndex+len(htmlcheck.PostLinkURLPrefix):]
			}
			p := 0
			for database.AmIsValidPostLinkChar(t[p]) {
				p++
			}
			if p > 0 {
				buf.WriteString("/go/")
				buf.WriteString(t[:p])
				t = t[p:]
			} else {
				buf.WriteString(htmlcheck.PostLinkURLPrefix)
			}
			plIndex = strings.Index(t, htmlcheck.PostLinkURLPrefix)
		}
		buf.WriteString(t)
		data = buf.String()
	}

	ulIndex = strings.Index(data, htmlcheck.UserLinkURIPRefix)
	if ulIndex >= 0 {
		var buf strings.Builder
		t := data
		for ulIndex >= 0 {
			if ulIndex > 0 {
				buf.WriteString(t[:ulIndex])
				t = t[ulIndex+len(htmlcheck.UserLinkURIPRefix):]
			}
			p := 0
			for database.AmIsValidAmsterdamIDChar(t[p]) {
				p++
			}
			if p > 0 {
				buf.WriteString("/user/")
				buf.WriteString(t[:p])
				t = t[p:]
			} else {
				buf.WriteString(htmlcheck.UserLinkURIPRefix)
			}
			ulIndex = strings.Index(t, htmlcheck.UserLinkURIPRefix)
		}
		buf.WriteString(t)
		data = buf.String()
	}
	return reflect.ValueOf(data)
}

// setupTemplates is called to set up the template renderer after the configuration is loaded.
func setupTemplates() {
	// Set up the template loaders: the optional filesystem loader, then the embedded loader.
	templateLoaders := make([]jet.Loader, 0, 2)
	if config.GlobalConfig.Resources.ViewTemplateDir != "" {
		finfo, err := os.Stat(config.GlobalConfig.Resources.ViewTemplateDir)
		if err == nil {
			if finfo.IsDir() {
				templateLoaders = append(templateLoaders, jet.NewOSFileSystemLoader(config.GlobalConfig.Resources.ViewTemplateDir))
			} else {
				log.Errorf("view template directory %s is not a directory, ignored", config.GlobalConfig.Resources.ViewTemplateDir)
			}
		} else {
			log.Errorf("view template directory %s is not valid, ignored (%v)", config.GlobalConfig.Resources.ViewTemplateDir, err)
		}
	}
	templateLoaders = append(templateLoaders, embedfs.NewLoader("views/", static_views))

	// Create the template renderer and add our globals to it.
	views = jet.NewSet(multi.NewLoader(templateLoaders...), jet.DevelopmentMode(true))
	views.AddGlobal("AmsterdamVersion", config.AMSTERDAM_VERSION)
	views.AddGlobal("AmsterdamCopyright", config.AMSTERDAM_COPYRIGHT)
	views.AddGlobal("GlobalConfig", config.GlobalConfig)
	views.AddGlobalFunc("iif", immediateIf)
	views.AddGlobalFunc("postRewrite", postRewrite)
	views.AddGlobalFunc("MakeIntRange", makeIntRange)
	views.AddGlobalFunc("MakeYearRange", makeYearRange)
	views.AddGlobalFunc("ExtractCommunityLogo", extractCommunityLogo)
	views.AddGlobalFunc("CommunityHost", communityHost)
	views.AddGlobalFunc("UserContactInfo", userContactInfo)
	views.AddGlobalFunc("DisplayActivity", displayActivity)
	views.AddGlobalFunc("DisplayDateTime", displayDateTime)
	views.AddGlobalFunc("DisplayMemberCount", displayMemberCount)
	views.AddGlobalFunc("DisplayFullName", displayFullName)
	views.AddGlobalFunc("DisplayExpandCat", displayExpandCat)

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

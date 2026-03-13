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

// Package ui holds the support for the Amsterdam user interface, wrapping Echo and Jet templates.
package ui

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"git.erbosoft.com/amy/amsterdam/config"
	"git.erbosoft.com/amy/amsterdam/database"
	"git.erbosoft.com/amy/amsterdam/util"
	"github.com/CloudyKit/jet/v6"
	"github.com/labstack/echo/v4"
	log "github.com/sirupsen/logrus"
)

/*----------------------------------------------------------------------------
 * AmContext interface
 *----------------------------------------------------------------------------
 */

const (
	FrameMetaHttpEquiv = 0 // <meta http-equiv="...">
)

// AmContext is the interface for Amsterdam's wrapper context that exposes the required functionality.
type AmContext interface {
	AddFrameMetadata(int, string, string)
	AddHeader(string, string)
	ClearCommunityContext()
	ClearLoginCookie()
	ClearSession()
	Ctx() context.Context
	CurrentCommunity() *database.Community
	CurrentUser() *database.User
	CurrentUserId() int32
	EffectiveLevel() uint16
	FormField(string) string
	FormFieldInt(string) (int, error)
	FormFieldIsSet(string) bool
	FormFieldValues(string) ([]string, error)
	FormFile(string) (*multipart.FileHeader, error)
	FrameTitle() string
	FrameMetadata(int) map[string]string
	Globals() *database.Globals
	GlobalFlags() *util.OptionSet
	HasParameter(string) bool
	IsMember() bool
	IsMemberLocked() bool
	LeftMenu() string
	Locator() string
	OutputType() string
	Parameter(string) string
	QueryParamInt(string, int) int
	RemoteIP() string
	ReplaceUser(*database.User)
	SaveSession() error
	SetCommunityContext(string) error
	SetFrameTitle(string)
	SetHeader(string, string)
	SetLeftMenu(string)
	SetLoginCookie(string)
	SetOutputType(string)
	GetScratch(string) any
	SetScratch(string, any)
	GetSession(string) any
	SetSession(string, any)
	IsSession(string) bool
	TestPermission(string) bool
	URLParam(string) string
	URLParamInt(string) (int, error)
	URLPath() string
	VarMap() jet.VarMap
	Verb() string
}

/*----------------------------------------------------------------------------
 * AmContext implementation
 *----------------------------------------------------------------------------
 */

// amContext is the internal structure that implements AmContext.
type amContext struct {
	echoContext    echo.Context
	rendervars     jet.VarMap
	frameTitle     string
	frameMeta      map[int]map[string]string
	outputType     string
	session        AmSession
	globals        *database.Globals
	globalFlags    *util.OptionSet
	user           *database.User
	effectiveLevel uint16
	community      *database.Community
	isMember       bool
	isMemberLocked bool
}

// AddFrameMetadata adds frame metadata of specified types.
func (c *amContext) AddFrameMetadata(selector int, name string, value string) {
	mv, ok := c.frameMeta[selector]
	if !ok {
		mv = make(map[string]string)
		c.frameMeta[selector] = mv
	}
	mv[name] = value
}

// AddHeader adds a header to the response.
func (c *amContext) AddHeader(key, value string) {
	c.echoContext.Response().Header().Add(key, value)
}

// ClearCommunityContext clears the community context so changes will be reflected.
func (c *amContext) ClearCommunityContext() {
	c.community = nil
	c.isMember = false
	c.isMemberLocked = false
	c.effectiveLevel = c.user.BaseLevel
}

// ClearLoginCookie overwrites and removes the login cookie.
func (c *amContext) ClearLoginCookie() {
	cookie := new(http.Cookie)
	cookie.Name = config.GlobalConfig.Site.LoginCookieName
	cookie.Value = ""
	cookie.Path = "/"
	cookie.Expires = time.Now()
	c.echoContext.SetCookie(cookie)
}

// ClearSession clears the current session.
func (c *amContext) ClearSession() {
	c.session.Reset(c.echoContext.Request().Context())
	c.user = nil
	c.effectiveLevel = 0
}

// Ctx returns the current context.Context for the request.
func (c *amContext) Ctx() context.Context {
	return c.echoContext.Request().Context()
}

// CurrentCommunity returns the current community, if one's been set.
func (c *amContext) CurrentCommunity() *database.Community {
	if c.community == nil {
		cv, ok := c.session.Get("lastCommunity")
		if ok && !c.CurrentUser().IsAnon {
			c.SetCommunityContext(fmt.Sprintf("%d", cv))
		}
	}
	return c.community
}

// CurrentUser returns the current user from the session.
func (c *amContext) CurrentUser() *database.User {
	if c.user == nil {
		id, ok := c.session.Uid()
		var err error
		var u *database.User
		if ok {
			u, err = database.AmGetUser(c.echoContext.Request().Context(), id)
		} else {
			u, err = database.AmGetAnonUser(c.echoContext.Request().Context())
		}
		if err != nil {
			log.Errorf("unable to retrieve current user")
		}
		c.user = u
		c.effectiveLevel = u.BaseLevel
	}
	return c.user
}

// CurrentUserId returns the current user ID.
func (c *amContext) CurrentUserId() int32 {
	if rc, ok := c.session.Uid(); ok {
		return rc
	}
	u, err := database.AmGetAnonUser(c.echoContext.Request().Context())
	if err == nil {
		return u.Uid
	}
	return 0
}

// EffectiveLevel returns the user's effective access level (in terms of current community, if any).
func (c *amContext) EffectiveLevel() uint16 {
	return c.effectiveLevel
}

/* FormField returns the value of a form field from the request.
 * Parameters:
 *     name - The name of the field to retrieve.
 * Returns:
 *     The value given to that named field.
 */
func (c *amContext) FormField(name string) string {
	return c.echoContext.FormValue(name)
}

/* FormFieldInt returns the value of a form field from the request, as an integer.
 * Parameters:
 *     name - The name of the field to retrieve.
 * Returns:
 *     The value given to that named field.
 *     Standard Go error status.
 */
func (c *amContext) FormFieldInt(name string) (int, error) {
	return strconv.Atoi(c.echoContext.FormValue(name))
}

/* FormFieldIsSet returns true if a given form field is set.
 * Parameters:
 *     name - The name of the field to test.
 * Returns:
 *     true if the field is set, false if not.
 */
func (c *amContext) FormFieldIsSet(name string) bool {
	req := c.echoContext.Request()
	if req.Form == nil {
		_ = req.FormValue(name) // force form to be loaded
	}
	return req.Form.Has(name)
}

// FormFieldValues returns all values for a specified parameter name.
func (c *amContext) FormFieldValues(name string) ([]string, error) {
	vals, err := c.echoContext.FormParams()
	if err != nil {
		return make([]string, 0), err
	}
	if rc, ok := vals[name]; ok {
		return rc, nil
	}
	return make([]string, 0), errors.New("parameter not found")
}

// FormFile returns a "file" parameter from a multipart upload form.
func (c *amContext) FormFile(name string) (*multipart.FileHeader, error) {
	return c.echoContext.FormFile(name)
}

// emptyMap is the return from FrameMetadata if the specified selector is not present.
var emptyMap map[string]string = make(map[string]string)

// FrameMetadata returns the frame metadata for a specified type.
func (c *amContext) FrameMetadata(selector int) map[string]string {
	rmap, ok := c.frameMeta[selector]
	if !ok {
		rmap = emptyMap
	}
	return rmap
}

// FrameTitle returns the frame title.
func (c *amContext) FrameTitle() string {
	return c.frameTitle
}

// Globals returns a reference to the database globals.
func (c *amContext) Globals() *database.Globals {
	return c.globals
}

// GlobalFlags returns a reference to the database global flags.
func (c *amContext) GlobalFlags() *util.OptionSet {
	return c.globalFlags
}

// HasParameter tests to see if we have a parameter.
func (c *amContext) HasParameter(name string) bool {
	s := c.echoContext.QueryParam(name)
	if s != "" {
		return true
	}
	s = c.echoContext.FormValue(name)
	if s != "" {
		return true
	}
	return false
}

// IsMember returns true if the user is a member of the current community.
func (c *amContext) IsMember() bool {
	return c.isMember
}

// IsMemberLocked returns true if the user is a "locked" member of the currentr community (cannot unjoin).
func (c *amContext) IsMemberLocked() bool {
	return c.isMemberLocked
}

// LeftMenu returns the current left menu selector.
func (c *amContext) LeftMenu() string {
	rc, ok := c.session.Get("leftMenu")
	if ok {
		return rc.(string)
	} else {
		return "top"
	}
}

// Locator returns the current URL path minus the scheme and host, so it's a site-relative locator.
func (c *amContext) Locator() string {
	tmp := url.URL{
		Path:        c.echoContext.Request().URL.Path,
		RawPath:     c.echoContext.Request().URL.RawPath,
		RawQuery:    c.echoContext.Request().URL.RawQuery,
		Fragment:    c.echoContext.Request().URL.Fragment,
		RawFragment: c.echoContext.Request().URL.RawFragment,
	}
	return tmp.String()
}

// OutputType returns the MIME output type set for the current operation.
func (c *amContext) OutputType() string {
	return c.outputType
}

/* Parameter returns the value of a parameter (query parameter or form field) from the request.
 * Parameters:
 *     name - The name of the field to retrieve.
 * Returns:
 *     The value given to that named field.
 */
func (c *amContext) Parameter(name string) string {
	rc := c.echoContext.QueryParam(name)
	if rc == "" {
		rc = c.echoContext.FormValue(name)
	}
	return rc
}

// QueryParamInt returns the value of a query parameter as an integer, with a default.
func (c *amContext) QueryParamInt(name string, defval int) int {
	s := c.echoContext.QueryParam(name)
	if s == "" {
		return defval
	}
	rc, err := strconv.Atoi(s)
	if err != nil {
		return defval
	}
	return rc
}

// RemoteIP returns the remote IP address.
func (c *amContext) RemoteIP() string {
	return c.echoContext.RealIP()
}

/* ReplaceUser replaces the current user in the context.
 * Parameters:
 *     u - New user to associate with the context.
 */
func (c *amContext) ReplaceUser(u *database.User) {
	c.session.SetUser(u)
	c.user = u
	c.effectiveLevel = u.BaseLevel
}

// SaveSession saves the session link to cookies.
func (c *amContext) SaveSession() error {
	return c.session.Save(c.echoContext.Request(), c.echoContext.Response())
}

/* SetCommunityContext establishes the community context from a (ID or alias) parameter.
 * Parameters:
 *     param - String parameter selecting the community.
 * Returns:
 *     Standard Go error status.
 */
func (c *amContext) SetCommunityContext(param string) error {
	comm, err := database.AmGetCommunityFromParam(c.echoContext.Request().Context(), param)
	if err != nil {
		return err
	}
	if c.community == nil || c.community.Id != comm.Id {
		mbr, lock, level, err := comm.Membership(c.echoContext.Request().Context(), c.CurrentUser())
		if err != nil {
			return err
		}
		c.community = comm
		c.isMember = mbr
		c.isMemberLocked = lock
		if level > c.effectiveLevel {
			c.effectiveLevel = level
		}
		if mbr {
			c.session.Set("lastCommunity", comm.Id)
		}
	}
	return nil
}

// SetFrameTitle sets the frame title for the output.
func (c *amContext) SetFrameTitle(s string) {
	c.frameTitle = s
}

// SetHeader sets a header on the output.
func (c *amContext) SetHeader(key, value string) {
	c.echoContext.Response().Header().Set(key, value)
}

// SetLeftMenu sets the current topmost left menu name value.
func (c *amContext) SetLeftMenu(name string) {
	c.session.Set("leftMenu", name)
}

/* SetLoginCookie adds the login cookie to the result output.
 * Parameters:
 *     auth - The auth string to set.
 */
func (c *amContext) SetLoginCookie(auth string) {
	cookie := new(http.Cookie)
	cookie.Name = config.GlobalConfig.Site.LoginCookieName
	cookie.Value = auth
	cookie.Path = "/"
	cookie.Expires = time.Now().AddDate(0, 0, config.GlobalConfig.Site.LoginCookieAge)
	c.echoContext.SetCookie(cookie)
}

// SetOutputType sets the MIME output type for the current operation.
func (c *amContext) SetOutputType(typ string) {
	c.outputType = typ
}

// GetScratch returns a value in the per-request scratchpad.
func (c *amContext) GetScratch(name string) any {
	return c.echoContext.Get("am." + name)
}

// SetScratch sets a value in the per-request scratchpad.
func (c *amContext) SetScratch(name string, val any) {
	c.echoContext.Set("am."+name, val)
}

// GetSession returns a session variable.
func (c *amContext) GetSession(name string) any {
	rc, _ := c.session.Get("x." + name)
	return rc
}

// SetSession sets a session variable.
func (c *amContext) SetSession(name string, value any) {
	c.session.Set("x."+name, value)
}

// IsSession tests to see whether a session value is set.
func (c *amContext) IsSession(name string) bool {
	_, ok := c.session.Get("x." + name)
	return ok
}

// TestPermission tests the current user against permissions.
func (c *amContext) TestPermission(perm string) bool {
	return database.AmTestPermission(perm, c.effectiveLevel)
}

// URLParam returns the value of a URL parameter.
func (c *amContext) URLParam(name string) string {
	return c.echoContext.Param(name)
}

// URLParamINt returns the value of a URL parameter parsed as an integer.
func (c *amContext) URLParamInt(name string) (int, error) {
	return strconv.Atoi(c.echoContext.Param(name))
}

// URLPath returns the path component of the request URL.
func (c *amContext) URLPath() string {
	return c.echoContext.Request().URL.Path
}

// VarMap provides access to the Jet variable map for setting variable data.
func (c *amContext) VarMap() jet.VarMap {
	return c.rendervars
}

// Verb returns the HTTP method (verb) for this request.
func (c *amContext) Verb() string {
	rc := c.echoContext.Request().Method
	if rc == "" {
		rc = "GET"
	}
	return rc
}

// defoptions is the default options for the HTTP session.
var defoptions *AmSessionOptions = &AmSessionOptions{
	Path:     "/",
	MaxAge:   86400,
	HttpOnly: true,
}

// freeContext is a free list for amContext structures.
var freeContext sync.Pool

// amContextRecycleBin is the channel we put contexts on to be recycled.
var amContextRecycleBin chan *amContext

/* newContext creates a new AmContext wrapping the Echo context.
 * Parameters:
 *     ctxt - The Echo context to be wrapped.
 * Returns:
 *     Internal Amsterdam context structure pointer, or nil.
 *     Standard Go error status.
 */
func newContext(ctxt echo.Context) (*amContext, error) {
	var rc *amContext
	tmp := freeContext.Get()
	if tmp == nil {
		rc = &amContext{
			rendervars: make(jet.VarMap),
			frameTitle: "",
			frameMeta:  make(map[int]map[string]string),
			outputType: "",
		}
	} else {
		rc = tmp.(*amContext)
	}

	var err error
	if rc.globals, err = database.AmGlobals(ctxt.Request().Context()); err != nil {
		amContextRecycleBin <- rc
		return nil, err
	}
	if rc.globalFlags, err = rc.globals.Flags(ctxt.Request().Context()); err != nil {
		amContextRecycleBin <- rc
		return nil, err
	}

	rc.echoContext = ctxt
	ctxt.Set("__amsterdam_context", rc)
	stmp := ctxt.Get("AmSessionStore")
	if stmp != nil {
		store := stmp.(AmSessionStore)
		sess, err := store.Get(ctxt.Request(), "AMSTERDAM_SESSION")
		if err == nil {
			rc.session = sess
			sess.SetOptions(defoptions)
			if sess.IsNew() {
				sess.FirstTime(ctxt.Request().Context())
			} else {
				sess.Hit()
			}
		}
		id, ok := sess.Uid()
		if ok {
			rc.user, err = database.AmGetUser(ctxt.Request().Context(), id)
			if err == nil {
				rc.effectiveLevel = rc.user.BaseLevel
			} else {
				rc.user = nil
				rc.effectiveLevel = database.AmRole("NotInList").Level()
			}
		} else {
			rc.user = nil
			rc.effectiveLevel = database.AmRole("NotInList").Level()
		}
		if rc.user != nil && !rc.user.IsAnon {
			cp, ok := sess.Get("lastCommunity")
			if ok {
				rc.SetCommunityContext(fmt.Sprintf("%d", cp))
			}
		}
	}
	return rc, err
}

/* AmContextFromEchoContext returns the AmContext associated with an Echo context.
 * Parameters:
 *     ctxt - The Echo context to have the AmContext extracted.
 * Returns:
 *     The associated AmContext.
 */
func AmContextFromEchoContext(ctxt echo.Context) AmContext {
	myctxt := ctxt.Get("__amsterdam_context")
	if myctxt != nil {
		rc, ok := myctxt.(*amContext)
		if ok {
			if rc.echoContext == nil {
				rc.echoContext = ctxt
			}
			return rc
		}
	}
	panic("Failed to find AmContext when required")
}

// contextRecycler is the task that recycles context blocks.
func contextRecycler(incoming chan *amContext, done chan bool) {
	for c := range incoming {
		c.echoContext = nil
		c.rendervars = make(jet.VarMap)
		c.frameTitle = ""
		c.frameMeta = make(map[int]map[string]string)
		c.outputType = ""
		c.session = nil
		c.globals = nil
		c.globalFlags = nil
		c.user = nil
		c.effectiveLevel = 0
		c.community = nil
		c.isMember = false
		c.isMemberLocked = false
		freeContext.Put(c)
	}
	done <- true
}

// setupContext starts the recycler for contexts.
func setupContext() func() {
	amContextRecycleBin = make(chan *amContext, config.GlobalConfig.Tuning.Queues.ContextRecycle)
	done := make(chan bool)
	go contextRecycler(amContextRecycleBin, done)
	return func() {
		close(amContextRecycleBin)
		<-done
	}
}

// ContextCreator is middleware that creates and recycles the AmContext.
func ContextCreator(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		myctxt, err := newContext(c)
		if err == nil {
			err = next(c)
			amContextRecycleBin <- myctxt
		}
		return err
	}
}

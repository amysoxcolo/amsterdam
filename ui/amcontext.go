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
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"strconv"
	"time"

	"git.erbosoft.com/amy/amsterdam/config"
	"git.erbosoft.com/amy/amsterdam/database"
	"git.erbosoft.com/amy/amsterdam/util"
	"github.com/CloudyKit/jet/v6"
	"github.com/gorilla/sessions"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
	log "github.com/sirupsen/logrus"
)

/*----------------------------------------------------------------------------
 * AmContext interface
 *----------------------------------------------------------------------------
 */

// AmContext is the interface for Amsterdam's wrapper context that exposes the required functionality.
type AmContext interface {
	ClearCommunityContext()
	ClearLoginCookie()
	ClearSession()
	CurrentCommunity() *database.Community
	CurrentUser() *database.User
	CurrentUserId() int32
	EffectiveLevel() uint16
	FormField(string) string
	FormFieldInt(string) (int, error)
	FormFieldIsSet(string) bool
	FormFile(string) (*multipart.FileHeader, error)
	Globals() *database.Globals
	GlobalFlags() *util.OptionSet
	IsMember() bool
	IsMemberLocked() bool
	LeftMenu() string
	RC() int
	OutputType() string
	Parameter(string) string
	QueryParamInt(string, int) int
	RemoteIP() string
	ReplaceUser(*database.User)
	SaveSession() error
	SubRender(string) ([]byte, error)
	SetCommunityContext(string) error
	SetLeftMenu(string)
	SetLoginCookie(string)
	SetOutputType(string)
	SetRC(int)
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
}

/*----------------------------------------------------------------------------
 * AmContext implementation
 *----------------------------------------------------------------------------
 */

// amContext is the internal structure that implements AmContext.
type amContext struct {
	echoContext    echo.Context
	httprc         int
	rendervars     jet.VarMap
	outputType     string
	session        *sessions.Session
	globals        *database.Globals
	globalFlags    *util.OptionSet
	user           *database.User
	effectiveLevel uint16
	community      *database.Community
	isMember       bool
	isMemberLocked bool
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
	AmResetSession(c.session)
	c.user = nil
	c.effectiveLevel = 0
}

// CurrentCommunity returns the current community, if one's been set.
func (c *amContext) CurrentCommunity() *database.Community {
	if c.community == nil {
		cv, ok := c.session.Values["lastCommunity"]
		if ok && !c.CurrentUser().IsAnon {
			c.SetCommunityContext(fmt.Sprintf("%d", cv))
		}
	}
	return c.community
}

// CurrentUser returns the current user from the session.
func (c *amContext) CurrentUser() *database.User {
	if c.user == nil {
		u, err := database.AmGetUser(AmSessionUid(c.session))
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
	return AmSessionUid(c.session)
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

// FormFile returns a "file" parameter from a multipart upload form.
func (c *amContext) FormFile(name string) (*multipart.FileHeader, error) {
	return c.echoContext.FormFile(name)
}

// Globals returns a reference to the database globals.
func (c *amContext) Globals() *database.Globals {
	return c.globals
}

// GlobalFlags returns a reference to the database global flags.
func (c *amContext) GlobalFlags() *util.OptionSet {
	return c.globalFlags
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
	return c.session.Values["leftMenu"].(string)
}

// RC returns the HTTP result code for the current operation.
func (c *amContext) RC() int {
	return c.httprc
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
	if rc == "" && c.echoContext.Request().Method == "POST" {
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
	AmSetSessionUser(c.session, u)
	c.user = u
	c.effectiveLevel = u.BaseLevel
}

// SaveSession saves the session link to cookies.
func (c *amContext) SaveSession() error {
	return c.session.Save(c.echoContext.Request(), c.echoContext.Response())
}

/* SubRender renders a subtemplate to the output.
 * Parameters:
 *	   name = The name of the template to be rendered.
 * Returns:
 *     Byte array with the rendered data to be output
 *     Standard Go error status
 */
func (c *amContext) SubRender(name string) ([]byte, error) {
	view, err := views.GetTemplate(name)
	if err != nil {
		log.Errorf("unable to load template \"%s\": %v", name, err)
		return nil, err
	}
	buf := new(bytes.Buffer)
	err = view.Execute(buf, c.VarMap(), c)
	if err != nil {
		log.Errorf("template \"%s\" failed subrender exec: %v", name, err)
	}
	return buf.Bytes(), err
}

/* SetCommunityContext establishes the community context from a (ID or alias) parameter.
 * Parameters:
 *     param - String parameter selecting the community.
 * Returns:
 *     Standard Go error status.
 */
func (c *amContext) SetCommunityContext(param string) error {
	comm, err := database.AmGetCommunityFromParam(param)
	if err != nil {
		return err
	}
	if c.community == nil || c.community.Id != comm.Id {
		mbr, lock, level, err := comm.Membership(c.CurrentUser())
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
			c.session.Values["lastCommunity"] = comm.Id
		}
	}
	return nil
}

// SetLeftMenu sets the current topmost left menu name value.
func (c *amContext) SetLeftMenu(name string) {
	c.session.Values["leftMenu"] = name
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

// SetRC sets the HTTP result code for the current operation.
func (c *amContext) SetRC(rc int) {
	c.httprc = rc
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
	return c.session.Values["x."+name]
}

// SetSession sets a session variable.
func (c *amContext) SetSession(name string, value any) {
	c.session.Values["x."+name] = value
}

// IsSession tests to see whether a session value is set.
func (c *amContext) IsSession(name string) bool {
	_, ok := c.session.Values["x."+name]
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

// defoptions is the default options for the HTTP session.
var defoptions *sessions.Options = &sessions.Options{
	Path:     "/",
	MaxAge:   86400,
	HttpOnly: true,
}

// freeContext is a free list for amContext structures.
var freeContext util.FreeList[amContext]

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
	rc := freeContext.Get()
	if rc == nil {
		rc = &amContext{
			httprc:     http.StatusOK,
			rendervars: make(jet.VarMap),
			outputType: "",
		}
	}

	var err error
	if rc.globals, err = database.AmGlobals(); err != nil {
		amContextRecycleBin <- rc
		return nil, err
	}
	if rc.globalFlags, err = rc.globals.Flags(); err != nil {
		amContextRecycleBin <- rc
		return nil, err
	}

	rc.echoContext = ctxt
	ctxt.Set("__amsterdam_context", rc)
	sess, err := session.Get("AMSTERDAM_SESSION", ctxt)
	if err == nil {
		rc.session = sess
		sess.Options = defoptions
		if sess.IsNew {
			AmSessionFirstTime(sess)
		} else {
			AmHitSession(sess)
		}
	}
	rc.user, err = database.AmGetUser(AmSessionUid(sess))
	if err == nil {
		rc.effectiveLevel = rc.user.BaseLevel
	} else {
		rc.user = nil
		rc.effectiveLevel = database.AmRole("NotInList").Level()
	}
	if !rc.user.IsAnon {
		cp, ok := sess.Values["lastCommunity"]
		if ok {
			rc.SetCommunityContext(fmt.Sprintf("%d", cp))
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
		rc, ok := myctxt.(AmContext)
		if ok {
			return rc
		}
	}
	panic("Failed to find AmContext when required")
}

// contextRecycler is the task that recycles context blocks.
func contextRecycler(incoming chan *amContext, done chan bool) {
	for c := range incoming {
		c.echoContext = nil
		c.httprc = http.StatusOK
		for k := range c.rendervars {
			delete(c.rendervars, k)
		}
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

// SetupAmContext starts the recycler for contexts.
func SetupAmContext() func() {
	amContextRecycleBin = make(chan *amContext, 16)
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

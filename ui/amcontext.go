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
	"net/http"
	"strconv"
	"time"

	"git.erbosoft.com/amy/amsterdam/config"
	"git.erbosoft.com/amy/amsterdam/database"
	"github.com/CloudyKit/jet/v6"
	"github.com/gorilla/sessions"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
	log "github.com/sirupsen/logrus"
)

// AmContext is the interface for Amsterdam's wrapper context that exposes the required functionality.
type AmContext interface {
	ClearLoginCookie()
	ClearSession()
	CurrentUser() *database.User
	CurrentUserId() int32
	FormField(string) string
	FormFieldInt(string) (int, error)
	FormFieldIsSet(string) bool
	RC() int
	OutputType() string
	Parameter(string) string
	RemoteIP() string
	Render(string) error
	ReplaceUser(*database.User)
	SaveSession() error
	SubRender(string) ([]byte, error)
	SetLoginCookie(string)
	SetOutputType(string)
	SetRC(int)
	GetScratch(string) any
	SetScratch(string, any)
	URLParam(string) string
	URLParamInt(string) (int, error)
	URLPath() string
	VarMap() jet.VarMap
}

// amContext is the internal structure that implements AmContext.
type amContext struct {
	echoContext echo.Context
	httprc      int
	rendervars  jet.VarMap
	outputType  string
	scratchpad  map[string]any
	session     *sessions.Session
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
	for k := range c.session.Values {
		delete(c.session.Values, k)
	}
	setupAmSession(c.session)
}

// CurrentUser returns the current user from the session.
func (c *amContext) CurrentUser() *database.User {
	u, err := database.AmGetUser(c.session.Values["user_id"].(int32))
	if err != nil {
		log.Errorf("unable to retrieve current user")
	}
	return u
}

// CurrentUserId returns the current user ID.
func (c *amContext) CurrentUserId() int32 {
	return c.session.Values["user_id"].(int32)
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

// RemoteIP returns the remote IP address.
func (c *amContext) RemoteIP() string {
	return c.echoContext.RealIP()
}

/* Render renders a template to the output. Called at the top level only.
 * Parameters:
 *     name = The name of the tempate to be rendered.
 * Returns:
 *	   Standard Go error status.
 */
func (c *amContext) Render(name string) error {
	return c.echoContext.Render(c.httprc, name, c)
}

/* ReplaceUser replaces the current user in the context.
 * Parameters:
 *     u - New user to associate with the context.
 */
func (c *amContext) ReplaceUser(u *database.User) {
	c.session.Values["user_id"] = u.Uid
}

// SaveSession saves the session link to cookies.
func (c *amContext) SaveSession() error {
	return c.session.Save(c.echoContext.Request(), c.echoContext.Response())
}

// Scratchpad returns the per-request scratchpad for values.
func (c *amContext) Scratchpad() map[string]any {
	if c.scratchpad == nil {
		c.scratchpad = make(map[string]any)
	}
	return c.scratchpad
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
	return buf.Bytes(), err
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
	if c.scratchpad == nil {
		return nil
	}
	return c.scratchpad[name]
}

// SetScratch sets a value in the per-request scratchpad.
func (c *amContext) SetScratch(name string, val any) {
	if c.scratchpad == nil {
		c.scratchpad = make(map[string]any)
	}
	c.scratchpad[name] = val
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

/* NewAmContext creates a new AmContext wrapping the Echo context.
 * Parameters:
 *     ctxt - The Echo context to be wrapped.
 * Returns:
 *     A new Amsterdam context wrapping that context.
 *     Standard Go error status.
 */
func NewAmContext(ctxt echo.Context) (AmContext, error) {
	rc := amContext{
		echoContext: ctxt,
		httprc:      http.StatusOK,
		rendervars:  make(jet.VarMap),
		outputType:  "",
		scratchpad:  nil,
	}
	ctxt.Set("amsterdam_context", &rc)
	sess, err := session.Get("amsterdam_session", ctxt)
	if err == nil {
		rc.session = sess
		sess.Options = defoptions
		if sess.IsNew {
			setupAmSession(sess)
		} else {
			log.Debugf("took the not-new-session path")
		}
	}
	return &rc, err
}

/* AmContextFromEchoContext returns the AmContext associated with an Echo context.
 * Parameters:
 *     ctxt - The Echo context to have the AmContext extracted.
 * Returns:
 *     The associated AmContext, or nil if there is none.
 */
func AmContextFromEchoContext(ctxt echo.Context) AmContext {
	myctxt := ctxt.Get("amsterdam_context")
	if myctxt != nil {
		rc, ok := myctxt.(AmContext)
		if ok {
			return rc
		}
	}
	return nil
}

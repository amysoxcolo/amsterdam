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

	"git.erbosoft.com/amy/amsterdam/database"
	"github.com/CloudyKit/jet/v6"
	"github.com/gorilla/sessions"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
	log "github.com/sirupsen/logrus"
)

// AmContext is the interface for Amsterdam's wapper context that exposes the required functionality.
type AmContext interface {
	CurrentUser() *database.User
	RC() int
	OutputType() string
	Render(string) error
	Scratchpad() map[any]any
	SubRender(string) ([]byte, error)
	Session() *sessions.Session
	SetOutputType(string)
	SetRC(int)
	URLPath() string
	VarMap() jet.VarMap
}

// amContext is the internal structure that implements AmContext.
type amContext struct {
	echoContext echo.Context
	httprc      int
	rendervars  jet.VarMap
	outputType  string
	scratchpad  map[any]any
	session     *sessions.Session
}

// CurrentUser returns the current user from the session.
func (c *amContext) CurrentUser() *database.User {
	u, err := database.AmGetUser(c.session.Values["user_id"].(int32))
	if err != nil {
		log.Errorf("unable to retrieve current user")
	}
	return u
}

// RC returns the HTTP result code for the current operation.
func (c *amContext) RC() int {
	return c.httprc
}

// OutputType returns the MIME output type set for the current operation.
func (c *amContext) OutputType() string {
	return c.outputType
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

// Scratchpad returns the per-request scratchpad for values.
func (c *amContext) Scratchpad() map[any]any {
	if c.scratchpad == nil {
		c.scratchpad = make(map[any]any)
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
		return nil, err
	}
	buf := new(bytes.Buffer)
	err = view.Execute(buf, c.VarMap(), c)
	return buf.Bytes(), err
}

// Session returns the HTTP session.
func (c *amContext) Session() *sessions.Session {
	return c.session
}

// SetOutputType sets the MIME output type for the current operation.
func (c *amContext) SetOutputType(typ string) {
	c.outputType = typ
}

// SetRC sets the HTTP result code for the current operation.
func (c *amContext) SetRC(rc int) {
	c.httprc = rc
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
			SetupAmSession(sess)
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

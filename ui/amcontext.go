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

	"github.com/CloudyKit/jet/v6"
	"github.com/labstack/echo/v4"
)

// AmContext is the interface for Amsterdam's wapper context that exposes the required functionality.
type AmContext interface {
	RC() int
	OutputType() string
	Render(string) error
	SubRender(string) ([]byte, error)
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

/* NewAmContext creates a new AmContext wrapping the Echo context.
 * Parameters:
 *     ctxt - The Echo context to be wrapped.
 * Returns:
 *     A new Amsterdam context wrapping that context.
 */
func NewAmContext(ctxt echo.Context) AmContext {
	rc := amContext{
		echoContext: ctxt,
		httprc:      http.StatusOK,
		rendervars:  make(jet.VarMap),
		outputType:  "",
	}
	ctxt.Set("amsterdam_context", &rc)
	return &rc
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

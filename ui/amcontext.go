/*
 * Amsterdam Web Communities System
 * Copyright (c) 2025 Erbosoft Metaverse Design Solutions, All Rights Reserved
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */
package ui

import (
	"bytes"
	"net/http"

	"github.com/CloudyKit/jet/v6"
	"github.com/labstack/echo/v4"
)

type AmContext interface {
	Render(string) error
	SubRender(string) ([]byte, error)
	SetRC(int)
	VarMap() jet.VarMap
}

type amContext struct {
	echoContext echo.Context
	httprc      int
	rendervars  jet.VarMap
}

func (c *amContext) Render(name string) error {
	return c.echoContext.Render(c.httprc, name, c)
}

func (c *amContext) SubRender(name string) ([]byte, error) {
	view, err := views.GetTemplate(name)
	if err != nil {
		return nil, err
	}
	buf := new(bytes.Buffer)
	err = view.Execute(buf, c.VarMap(), c)
	return buf.Bytes(), err
}

func (c *amContext) SetRC(rc int) {
	c.httprc = rc
}

func (c *amContext) VarMap() jet.VarMap {
	return c.rendervars
}

func NewAmContext(ctxt echo.Context) AmContext {
	rc := amContext{
		echoContext: ctxt,
		httprc:      http.StatusOK,
		rendervars:  make(jet.VarMap),
	}
	ctxt.Set("amsterdam_context", &rc)
	return &rc
}

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

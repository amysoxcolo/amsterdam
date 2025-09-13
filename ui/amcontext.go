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
	"net/http"

	"github.com/labstack/echo/v4"
)

type AmContext interface {
	Render(string) error
	SetRC(int)
}

type amContext struct {
	echoContext echo.Context
	httprc      int
}

func (c *amContext) Render(name string) error {
	return c.echoContext.Render(c.httprc, name, c)
}

func (c *amContext) SetRC(rc int) {
	c.httprc = rc
}

func NewAmContext(ctxt echo.Context) AmContext {
	rc := amContext{
		echoContext: ctxt,
		httprc:      http.StatusOK,
	}
	return &rc
}

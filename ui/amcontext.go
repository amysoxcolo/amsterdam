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
	"github.com/labstack/echo/v4"
)

type AmContext interface {
	Render(int, string, interface{}) error
}

type amContext struct {
	echoContext echo.Context
}

func (c *amContext) Render(code int, name string, data interface{}) error {
	return c.echoContext.Render(code, name, data)
}

func NewAmContext(ctxt echo.Context) AmContext {
	rc := amContext{
		echoContext: ctxt,
	}
	return &rc
}

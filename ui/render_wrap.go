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
	"fmt"

	"github.com/labstack/echo/v4"
)

func AmWrap(myfunc func(AmContext) (string, any, error)) echo.HandlerFunc {
	return func(ctxt echo.Context) error {
		amctxt := NewAmContext(ctxt)
		what, rc, err := myfunc(amctxt)
		if err == nil {
			switch what {
			case "template":
				err = amctxt.Render(fmt.Sprintf("%v", rc))
			default:
				err = fmt.Errorf("unknown rendering type: %s", what)
			}
		}
		return err
	}
}

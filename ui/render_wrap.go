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
	"fmt"

	"github.com/labstack/echo/v4"
)

func sendPageData(ctxt echo.Context, amctxt AmContext, command string, data any) error {
	var err error
	switch command {
	case "bytes":
		err = ctxt.Blob(amctxt.RC(), amctxt.OutputType(), data.([]byte))
	case "string":
		err = ctxt.String(amctxt.RC(), fmt.Sprintf("%v", data))
	case "template":
		err = amctxt.Render(fmt.Sprintf("%v", data))
	case "framed_template":
		amctxt.VarMap().Set("amsterdam_innerPage", data)
		err = amctxt.Render("frame.jet")
	default:
		err = fmt.Errorf("unknown rendering type: %s", command)
	}
	return err
}

/* ErrorPage renders the Amsterdam page with a server error message.
 * Parameters:
 *     ctxt - The AmContext for the request.
 *     input_err - The error to be rendered on the page.
 * Returns:
 *     Command string dictating what to be rendered.
 *     Data as a parameter for the command string.
 *     Standard Go error status.
 */
func ErrorPage(ctxt AmContext, input_err error) (string, any, error) {
	ctxt.VarMap().Set("amsterdam_pageTitle", "Internal Server Error")
	ctxt.VarMap().Set("error", input_err.Error())
	return "framed_template", "error.jet", nil
}

/* AmWrap wraps the Amsterdam handler function in a wrapper that implements the spec for
 * Echo handler functions.
 * Parameters:
 *     myfunc - The Amsterdam handler to be wrapped.
 * Returns:
 *     The wrapped function.
 */
func AmWrap(myfunc func(AmContext) (string, any, error)) echo.HandlerFunc {
	return func(ctxt echo.Context) error {
		amctxt, aerr := NewAmContext(ctxt)
		if aerr != nil {
			ctxt.Logger().Errorf("Session creation error: %v", aerr)
			return aerr
		}
		what, rc, err := myfunc(amctxt)
		if err == nil {
			if err = amctxt.Session().Save(ctxt.Request(), ctxt.Response()); err != nil {
				ctxt.Logger().Errorf("Session save error: %v", err)
				return err
			}
			err = sendPageData(ctxt, amctxt, what, rc)
			if err != nil {
				ctxt.Logger().Errorf("Rendering error: %v", err)
			}
		} else {
			ctxt.Logger().Errorf("Page function error: %v", err)
			_, rc, _ = ErrorPage(amctxt, err)
			newerr := sendPageData(ctxt, amctxt, "framed_template", rc)
			err = newerr
		}
		return err
	}
}

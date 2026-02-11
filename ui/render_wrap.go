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
	"net/http"
	"time"

	"github.com/klauspost/lctime"
	"github.com/labstack/echo/v4"
	log "github.com/sirupsen/logrus"
)

/* AmSendPageData sends page data to the output based on the command string.
 * Parameters:
 *     ctxt - The Echo context from the request.
 *     amctxt - The associated AmContext.
 *     command - The type of rendering to be done. Known values are:
 *         "bytes" - Output "data" as a byte array.
 *         "redirect" - Treat "data" as a URL to be redirected to and send a 302 Redirect.
 *         "string" - Output "data" as a string.
 *         "template" - Treat "data" as a template name, and output that template.
 *         "framed" - Treat "data" as an inner template name, and output that template rendered
 *		       within the outer "frame.jet" template.
 *     data - The data to be output, as determined by the command.
 * Returns:
 *     Standard Go error status.
 */
func AmSendPageData(ctxt echo.Context, amctxt AmContext, command string, data any) error {
	var err error

	if command == "error" {
		httprc := amctxt.RC()
		message := ""
		if data == nil {
			message = fmt.Sprintf("Unspecified error in %s", ctxt.Request().URL.String())
		} else if he, ok := data.(*echo.HTTPError); ok {
			httprc = he.Code
			m1 := he.Message
			e1 := he.Unwrap()
			if m1 == nil || m1 == "" {
				if e1 == nil {
					message = fmt.Sprintf("Unspecified error in %s", ctxt.Request().URL.String())
				} else {
					message = e1.Error()
				}
			} else {
				if e1 == nil {
					message = fmt.Sprintf("%v", m1)
				} else {
					message = fmt.Sprintf("%v (%v)", m1, e1)
				}
			}
		} else if er, ok := data.(error); ok {
			message = er.Error()
		} else {
			message = fmt.Sprintf("%v", data)
		}
		if httprc < 400 {
			httprc = http.StatusInternalServerError
		}
		amctxt.VarMap().Set("amsterdam_pageTitle", "Internal Server Error")
		amctxt.VarMap().Set("error", message)
		amctxt.SetRC(httprc)
		command = "framed"
		data = "error.jet"
	}

	switch command {
	case "bytes":
		err = ctxt.Blob(amctxt.RC(), amctxt.OutputType(), data.([]byte))
	case "redirect":
		err = ctxt.Redirect(http.StatusFound, data.(string))
	case "string":
		err = ctxt.String(amctxt.RC(), data.(string))
	case "template":
		err = ctxt.Render(amctxt.RC(), data.(string), amctxt)
	case "framed":
		amctxt.VarMap().Set("amsterdam_innerPage", data)
		menus := make([]*MenuDefinition, 2)
		switch amctxt.LeftMenu() {
		case "top":
			menus[0] = AmMenu("top")
		case "community":
			md, err := AmBuildCommunityMenu(ctxt.Request().Context(), amctxt.CurrentCommunity())
			if err != nil {
				return err
			}
			menus[0] = md
		default:
			return fmt.Errorf("AmSendPageData(): unknown left menu context: %s", amctxt.LeftMenu())
		}
		menus[1] = AmMenu("fixed")
		amctxt.VarMap().Set("amsterdam_leftMenus", menus)
		err = ctxt.Render(amctxt.RC(), "frame.jet", amctxt)
	default:
		err = fmt.Errorf("AmSendPageData(): unknown rendering type: %s", command)
	}
	if err != nil {
		log.Errorf("AmSendPageData() barfed with %v", err)
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
	if input_err == nil {
		log.Error("ErrorPage called with nil input error, WTF?")
	}
	ctxt.VarMap().Set("amsterdam_pageTitle", "Internal Server Error")
	ctxt.VarMap().Set("error", input_err.Error())
	return "framed", "error.jet", nil
}

// expireTime is the expiration time sent in the dynamic headers.
var expireTime string = lctime.Strftime("%c", time.Unix(1, 0))

type PageFunc func(AmContext) (string, any)

/* AmWrap wraps the Amsterdam handler function in a wrapper that implements the spec for
 * Echo handler functions.
 * Parameters:
 *     myfunc - The Amsterdam handler to be wrapped.
 * Returns:
 *     The wrapped function.
 */
func AmWrap(myfunc PageFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctxt := AmContextFromEchoContext(c)

		// Add the dynamic headers.
		c.Response().Header().Set("Pragma", "No-cache")
		c.Response().Header().Set("Cache-Control", "no-cache")
		c.Response().Header().Set("Expires", expireTime)

		// Exec the wrapped function.
		command, arg := myfunc(ctxt)
		if err := ctxt.SaveSession(); err != nil {
			c.Logger().Errorf("Session save error: %v", err)
			return err
		}
		if err := AmSendPageData(c, ctxt, command, arg); err != nil {
			c.Logger().Errorf("Rendering error: %v", err)
			return err
		}
		return nil
	}
}

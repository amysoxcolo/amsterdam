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
	"io"
	"net/http"
	"time"

	"git.erbosoft.com/amy/amsterdam/config"
	"git.erbosoft.com/amy/amsterdam/database"
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
 *         "error" - Output the error rendering page.
 *         "framed" - Treat "data" as an inner template name, and output that template rendered
 *		       within the outer "frame.jet" template.
 *         "ipban" - Output the IP address ban rendering page.
 *         "nocontent" - Output a 204 No Content response.
 *         "redirect" - Treat "data" as a URL to be redirected to and send a 302 Redirect.
 *         "stream" - Treat "data" as an io.Reader and use it to stream data.
 *         "string" - Output "data" as a string.
 *         "template" - Treat "data" as a template name, and output that template.
 *     data - The data to be output, as determined by the command.
 * Returns:
 *     Standard Go error status.
 */
func AmSendPageData(ctxt echo.Context, amctxt AmContext, command string, data any) error {
	// Preprocess certain commands into different ones.
	httprc := http.StatusOK
	switch command {
	case "error":
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
		amctxt.SetFrameTitle("Internal Server Error")
		amctxt.VarMap().Set("error", message)
		if tmp := amctxt.GetSession("lastKnownGood"); tmp != nil {
			amctxt.VarMap().Set("recovery", tmp)
		}
		command = "framed"
		data = "error.jet"
	case "ipban":
		amctxt.SetFrameTitle("IP Address Banned")
		amctxt.VarMap().Set("message", data)
		httprc = http.StatusForbidden
		command = "framed"
		data = "ipban.jet"
	case "ratelimit":
		amctxt.SetFrameTitle("Rate Limit Exceeded")
		httprc = http.StatusTooManyRequests
		command = "framed"
		data = "ratelimit.jet"
	}

	// Process commands.
	var err error
	switch command {
	case "bytes":
		err = ctxt.Blob(httprc, amctxt.OutputType(), data.([]byte))
	case "stream":
		err = ctxt.Stream(httprc, amctxt.OutputType(), data.(io.Reader))
	case "redirect":
		err = ctxt.Redirect(http.StatusFound, data.(string))
	case "nocontent":
		err = ctxt.NoContent(http.StatusNoContent)
	case "string":
		err = ctxt.String(httprc, data.(string))
	case "template":
		err = ctxt.Render(httprc, data.(string), amctxt)
	case "framed":
		if amctxt.FrameTitle() == "" {
			ctxt.Logger().Errorf("*** NO FRAME TITLE set for path %s", amctxt.URLPath())
			amctxt.SetFrameTitle("<<< NO FRAME TITLE >>>")
		}
		amctxt.VarMap().Set("__innerPage", data)
		menus := make([]*MenuDefinition, 2)
		switch amctxt.LeftMenu() {
		case "top":
			menus[0] = AmMenu("top")
		case "community":
			comm := amctxt.CurrentCommunity()
			if comm != nil {
				md, err := AmBuildCommunityMenu(ctxt.Request().Context(), comm)
				if err != nil {
					return err
				}
				menus[0] = md
			} else {
				menus[0] = AmMenu("top")
			}
		default:
			return fmt.Errorf("AmSendPageData(): unknown left menu context: %s", amctxt.LeftMenu())
		}
		menus[1] = AmMenu("fixed")
		amctxt.VarMap().Set("__leftMenus", menus)
		ad, err := database.AmGetRandomAd(ctxt.Request().Context())
		if err != nil {
			ad = &database.Advert{
				AdId:      -1,
				ImagePath: "",
				PathStyle: -1,
				Caption:   nil,
				LinkURL:   nil,
			}
		}
		amctxt.VarMap().Set("__bannerad", ad)
		amctxt.VarMap().Set("__debugMode", config.GlobalComputedConfig.DebugMode)
		if tmp := amctxt.GetScratch("frame_suppressLogin"); tmp != nil {
			amctxt.VarMap().Set("__suppressLogin", true)
		}
		err = ctxt.Render(httprc, "frame.jet", amctxt)
	default:
		err = fmt.Errorf("AmSendPageData(): unknown rendering type: %s", command)
	}
	if err != nil {
		log.Errorf("AmSendPageData() barfed with %v", err)
	}
	return err
}

// expireTime is the expiration time sent in the dynamic headers.
var expireTime string = lctime.Strftime("%c", time.Unix(1, 0))

// AmPageFunc is the definition for an Amsterdam "page function" that handles most of the work and defers to the wrapper for rendering.
type AmPageFunc func(AmContext) (string, any)

/* AmWrap wraps the Amsterdam handler function in a wrapper that implements the spec for
 * Echo handler functions.
 * Parameters:
 *     myfunc - The Amsterdam handler to be wrapped.
 * Returns:
 *     The wrapped function.
 */
func AmWrap(myfunc AmPageFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctxt := AmContextFromEchoContext(c)

		// Add the dynamic headers.
		c.Response().Header().Set("Pragma", "No-cache")
		c.Response().Header().Set("Cache-Control", "no-cache")
		c.Response().Header().Set("Expires", expireTime)

		// Exec the wrapped function.
		command, arg := myfunc(ctxt)
		if command != "error" && command != "ipban" {
			ctxt.SetSession("lastKnownGood", ctxt.Locator())
		}
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

// AmWithTempContext runs a page function with a temporary context. Used in error handling.
func AmWithTempContext(c echo.Context, fn AmPageFunc) error {
	var ctxt AmContext = nil
	myctxt := c.Get("__amsterdam_context")
	if myctxt != nil {
		ac, ok := myctxt.(*amContext)
		if ok {
			ctxt = ac
			ac.echoContext = c
		}
	}
	if ctxt == nil {
		ac, err := newContext(c)
		if err != nil {
			return err
		}
		ctxt = ac
		defer func() {
			amContextRecycleBin <- ac
		}()
	}

	// Call the function
	command, arg := fn(ctxt)

	// Add the dynamic headers.
	c.Response().Header().Set("Pragma", "No-cache")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Expires", expireTime)

	if err := AmSendPageData(c, ctxt, command, arg); err != nil {
		c.Logger().Errorf("Rendering error: %v", err)
		return err
	}
	return nil
}

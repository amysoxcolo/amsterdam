/*
 * Amsterdam Web Communities System
 * Copyright (c) 2025-2026 Erbosoft Metaverse Design Solutions, All Rights Reserved
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * SPDX-License-Identifier: MPL-2.0
 */

// Package ui holds the support for the Amsterdam user interface, wrapping Echo and Jet templates.
package ui

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"time"

	"git.erbosoft.com/amy/amsterdam/config"
	"git.erbosoft.com/amy/amsterdam/database"
	"github.com/klauspost/lctime"
	"github.com/labstack/echo/v5"
	log "github.com/sirupsen/logrus"
)

// panicRecoveryErr is the error created for panic recovery.
type panicRecoveryErr struct {
	Phase string // phase of operation
	Err   error  // error value
	Stack []byte // stack trace
}

// Error returns the actual error string.
func (e *panicRecoveryErr) Error() string {
	return fmt.Sprintf("[Panic Recovery in %s Phase] %s %s", e.Phase, e.Err.Error(), e.Stack)
}

// Unwrap returns the error "nested" inside this error.
func (e *panicRecoveryErr) Unwrap() error {
	return e.Err
}

// doFrameRender renders the outer frame template with an inner template.
func doFrameRender(ctxt *echo.Context, amctxt AmContext, statusCode int, innerPage string) error {
	if amctxt.FrameTitle() == "" {
		log.Errorf("*** NO FRAME TITLE set for path %s", amctxt.URLPath())
		amctxt.SetFrameTitle("<<< NO FRAME TITLE >>>")
	}
	amctxt.VarMap().Set("__innerPage", innerPage)
	menus := make([]*MenuDefinition, 2)
	switch amctxt.LeftMenu() {
	case "top":
		menus[0] = AmMenu(config.GlobalConfig.Site.TopMenuId)
	case "community":
		comm := amctxt.CurrentCommunity()
		if comm != nil {
			md, err := AmBuildCommunityMenu(ctxt.Request().Context(), comm)
			if err != nil {
				return err
			}
			menus[0] = md
		} else {
			menus[0] = AmMenu(config.GlobalConfig.Site.TopMenuId)
		}
	default:
		return fmt.Errorf("AmSendPageData(): unknown left menu context: %s", amctxt.LeftMenu())
	}
	menus[1] = AmMenu(config.GlobalConfig.Site.FixedMenuId)
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
	return ctxt.Render(statusCode, config.GlobalConfig.Site.FrameTemplate, amctxt)
}

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
func AmSendPageData(ctxt *echo.Context, amctxt AmContext, command string, data any) error {
	// Enable panic recovery.
	if !config.CommandLine.DebugPanic {
		defer func() {
			if r := recover(); r != nil {
				if r == http.ErrAbortHandler {
					panic(r)
				}
				tmperr, ok := r.(error)
				if !ok {
					tmperr = fmt.Errorf("%v", r)
				}
				stack := make([]byte, config.GlobalComputedConfig.PanicRecoveryStack)
				length := runtime.Stack(stack, false)
				log.Errorf("[Panic Recovery in SendData Phase] %s %s", tmperr.Error(), stack[:length])
			}
		}()
	}

	// Preprocess certain commands into different ones.
	httprc := http.StatusOK
	switch command {
	case "error":
		message := fmt.Sprintf("Unspecified error in %s", ctxt.Request().URL.String())
		if data != nil {
			if he, ok := data.(*echo.HTTPError); ok {
				httprc = he.Code
				m1 := he.Message
				e1 := he.Unwrap()
				if m1 == "" {
					if e1 != nil {
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
		}
		if httprc < 400 {
			httprc = http.StatusInternalServerError
		}
		amctxt.SetFrameTitle(http.StatusText(httprc))
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
	oldreq := ctxt.Request()
	ctx, cancel := context.WithTimeout(oldreq.Context(), time.Duration(config.GlobalConfig.Tuning.Timeouts.PageRender)*time.Second)
	defer cancel()
	ctxt.SetRequest(oldreq.WithContext(ctx))
	defer ctxt.SetRequest(oldreq)
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
		err = doFrameRender(ctxt, amctxt, httprc, data.(string))
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

// callWrappedPageFunc calls the specified page functon inside a wrapper that handles timeouts and panic recovery.
func callWrappedPageFunc(f AmPageFunc, ctxt *echo.Context, amctxt AmContext) (command string, arg any) {
	if !config.CommandLine.DebugPanic {
		defer func() {
			if r := recover(); r != nil {
				if r == http.ErrAbortHandler {
					panic(r)
				}
				tmperr, ok := r.(error)
				if !ok {
					tmperr = fmt.Errorf("%v", r)
				}
				stack := make([]byte, config.GlobalComputedConfig.PanicRecoveryStack)
				length := runtime.Stack(stack, false)
				arg = &panicRecoveryErr{Phase: "PageFunc", Err: tmperr, Stack: stack[:length]}
				command = "error"
			}
		}()
	}
	oldreq := ctxt.Request()
	ctx, cancel := context.WithTimeout(oldreq.Context(), time.Duration(config.GlobalConfig.Tuning.Timeouts.PageExecute)*time.Second)
	defer cancel()
	ctxt.SetRequest(oldreq.WithContext(ctx))
	defer ctxt.SetRequest(oldreq)
	command, arg = f(amctxt)
	return
}

/* AmWrap wraps the Amsterdam handler function in a wrapper that implements the spec for
 * Echo handler functions.
 * Parameters:
 *     myfunc - The Amsterdam handler to be wrapped.
 * Returns:
 *     The wrapped function.
 */
func AmWrap(myfunc AmPageFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		ctxt := AmContextFromEchoContext(c)

		// Add the dynamic headers.
		c.Response().Header().Set("Pragma", "No-cache")
		c.Response().Header().Set("Cache-Control", "no-cache")
		c.Response().Header().Set("Expires", expireTime)

		// Exec the wrapped function.
		command, arg := callWrappedPageFunc(myfunc, c, ctxt)
		if command != "error" && command != "ipban" {
			ctxt.SetSession("lastKnownGood", ctxt.Locator())
		}
		if err := ctxt.SaveSession(); err != nil {
			log.Errorf("Session save error: %v", err)
			return err
		}
		if err := AmSendPageData(c, ctxt, command, arg); err != nil {
			log.Errorf("Rendering error: %v", err)
			return err
		}
		return nil
	}
}

// AmWithTempContext runs a page function with a temporary context. Used in error handling.
func AmWithTempContext(c *echo.Context, fn AmPageFunc) error {
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
	command, arg := callWrappedPageFunc(fn, c, ctxt)

	// Add the dynamic headers.
	c.Response().Header().Set("Pragma", "No-cache")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Expires", expireTime)

	if err := AmSendPageData(c, ctxt, command, arg); err != nil {
		log.Errorf("Rendering error: %v", err)
		return err
	}
	return nil
}

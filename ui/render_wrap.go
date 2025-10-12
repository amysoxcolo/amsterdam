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
	"net/url"

	"git.erbosoft.com/amy/amsterdam/config"
	"git.erbosoft.com/amy/amsterdam/database"
	"github.com/labstack/echo/v4"
	log "github.com/sirupsen/logrus"
)

func sendPageData(ctxt echo.Context, amctxt AmContext, command string, data any) error {
	var err error
	switch command {
	case "bytes":
		err = ctxt.Blob(amctxt.RC(), amctxt.OutputType(), data.([]byte))
	case "redirect":
		err = ctxt.Redirect(http.StatusFound, data.(string))
	case "string":
		err = ctxt.String(amctxt.RC(), fmt.Sprintf("%v", data))
	case "template":
		err = amctxt.Render(fmt.Sprintf("%v", data))
	case "framed_template":
		amctxt.VarMap().Set("amsterdam_innerPage", data)
		augmentWithLeftMenus(amctxt)
		err = amctxt.Render("frame.jet")
	default:
		err = fmt.Errorf("unknown rendering type: %s", command)
	}
	if err != nil {
		log.Errorf("sendPageData() barfed with %v", err)
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
		// Create the AmContext.
		amctxt, aerr := NewAmContext(ctxt)
		if aerr != nil {
			ctxt.Logger().Errorf("Session creation error: %v", aerr)
			return aerr
		}

		// Check IP banning.
		banmsg, banerr := database.AmTestIPBan(ctxt.RealIP())
		if banerr != nil {
			ctxt.Logger().Warnf("address %s could not be tested: %v", ctxt.RealIP(), banerr)
			// but let the request pass anyway
		} else if banmsg != "" {
			amctxt.VarMap().Set("amsterdam_pageTitle", "IP Address Banned")
			amctxt.VarMap().Set("message", banmsg)
			amctxt.SetRC(http.StatusForbidden)
			return sendPageData(ctxt, amctxt, "framed_template", "ipban.jet")
		}

		// Check for cookie login.
		if amctxt.CurrentUser().IsAnon {
			cookie, cerr := ctxt.Cookie(config.GlobalConfig.Site.LoginCookieName)
			if cerr == nil {
				var user *database.User
				user, cerr = database.AmAuthenticateUserByToken(cookie.Value, ctxt.RealIP())
				if cerr == nil {
					// log the user in and rotate login cookie
					amctxt.ReplaceUser(user)
					var newToken string
					if newToken, cerr = user.NewAuthToken(); cerr == nil {
						amctxt.SetLoginCookie(newToken)
					} else {
						log.Warnf("unable to rotate login cookie: %v", cerr)
					}
					if !user.VerifyEMail {
						// bounce to E-mail verification before we go anywhere
						return sendPageData(ctxt, amctxt, "redirect",
							"/verify?tgt="+url.QueryEscape(ctxt.Request().URL.Path))
					}
				} else {
					log.Errorf("login cookie bogus, do not use: %v", cerr)
					amctxt.ClearLoginCookie()
				}
			}
		}

		// Exec the wrapped function.
		what, rc, err := myfunc(amctxt)
		if err == nil {
			if err = amctxt.SaveSession(); err != nil {
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
			amctxt.SetRC(http.StatusInternalServerError)
			newerr := sendPageData(ctxt, amctxt, "framed_template", rc)
			err = newerr
		}
		return err
	}
}

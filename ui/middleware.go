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
	"net/http"
	"net/url"

	"git.erbosoft.com/amy/amsterdam/config"
	"git.erbosoft.com/amy/amsterdam/database"
	"github.com/labstack/echo/v4"
	log "github.com/sirupsen/logrus"
)

// IPBanTest is middleware that handles the IP banning.
func IPBanTest(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		// Check IP banning.
		banmsg, banerr := database.AmTestIPBan(c.RealIP())
		if banerr != nil {
			c.Logger().Warnf("address %s could not be tested: %v", c.RealIP(), banerr)
			// but let the request pass anyway
		} else if banmsg != "" {
			amctxt := AmContextFromEchoContext(c)
			amctxt.VarMap().Set("amsterdam_pageTitle", "IP Address Banned")
			amctxt.VarMap().Set("message", banmsg)
			amctxt.SetRC(http.StatusForbidden)
			return AmSendPageData(c, amctxt, "framed_template", "ipban.jet")
		}
		return next(c)
	}
}

// CookieLoginTest is middleware that handles cookie logins.
func CookieLoginTest(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		amctxt := AmContextFromEchoContext(c)
		// Check for cookie login.
		if amctxt.CurrentUser().IsAnon {
			cookie, err := c.Cookie(config.GlobalConfig.Site.LoginCookieName)
			if err == nil {
				var user *database.User
				user, err = database.AmAuthenticateUserByToken(cookie.Value, c.RealIP())
				if err == nil {
					// log the user in and rotate login cookie
					amctxt.ReplaceUser(user)
					var newToken string
					if newToken, err = user.NewAuthToken(); err == nil {
						amctxt.SetLoginCookie(newToken)
					} else {
						log.Warnf("unable to rotate login cookie: %v", err)
					}
					if !user.VerifyEMail {
						// bounce to E-mail verification before we go anywhere
						return AmSendPageData(c, amctxt, "redirect",
							"/verify?tgt="+url.QueryEscape(c.Request().URL.Path))
					}
				} else {
					log.Errorf("login cookie bogus, do not use: %v", err)
					amctxt.ClearLoginCookie()
				}
			}
		}
		return next(c)
	}
}

// SetCommunity is middleware that sets the community context based on the URL.
func SetCommunity(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctxt := AmContextFromEchoContext(c)
		err := ctxt.SetCommunityContext(ctxt.URLParam("cid"))
		if err != nil {
			ctxt.SetRC(http.StatusNotFound)
			cmd, data, _ := ErrorPage(ctxt, err)
			return AmSendPageData(c, ctxt, cmd, data)
		}
		ctxt.SetLeftMenu("community")
		return next(c)
	}
}

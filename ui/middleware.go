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
	"strconv"

	"git.erbosoft.com/amy/amsterdam/config"
	"git.erbosoft.com/amy/amsterdam/database"
	"github.com/labstack/echo/v4"
	log "github.com/sirupsen/logrus"
)

// IPBanTest is middleware that handles the IP banning.
func IPBanTest(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		// Check IP banning.
		banmsg, banerr := database.AmTestIPBan(c.Request().Context(), c.RealIP())
		if banerr != nil {
			c.Logger().Warnf("address %s could not be tested: %v", c.RealIP(), banerr)
			// but let the request pass anyway
		} else if banmsg != "" {
			amctxt := AmContextFromEchoContext(c)
			amctxt.VarMap().Set("amsterdam_pageTitle", "IP Address Banned")
			amctxt.VarMap().Set("message", banmsg)
			amctxt.SetRC(http.StatusForbidden)
			return AmSendPageData(c, amctxt, "framed", "ipban.jet")
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
				user, err = database.AmAuthenticateUserByToken(c.Request().Context(), cookie.Value, c.RealIP())
				if err == nil {
					// log the user in and rotate login cookie
					amctxt.ReplaceUser(user)
					var newToken string
					if newToken, err = user.NewAuthToken(c.Request().Context()); err == nil {
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
			return AmSendPageData(c, ctxt, "error", echo.NewHTTPError(http.StatusNotFound).SetInternal(err))
		}
		ctxt.SetLeftMenu("community")
		return next(c)
	}
}

// ValidateConference is middleware that validates the user has access to the community's conference facility.
func ValidateConference(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctxt := AmContextFromEchoContext(c)
		comm := ctxt.CurrentCommunity() // set by middleware
		b, err := database.AmTestService(c.Request().Context(), comm, "Conference")
		if err != nil {
			return AmSendPageData(c, ctxt, "error", err)
		}
		if !b {
			return AmSendPageData(c, ctxt, "error", echo.NewHTTPError(http.StatusNotFound, "this community does not use conferencing services"))
		}
		if comm.MembersOnly && !ctxt.IsMember() && !ctxt.TestPermission("Community.NoJoinRequired") {
			return AmSendPageData(c, ctxt, "error", echo.NewHTTPError(http.StatusForbidden, "you are not a member of this community"))
		}
		if !comm.TestPermission("Community.Read", ctxt.EffectiveLevel()) {
			return AmSendPageData(c, ctxt, "error", echo.NewHTTPError(http.StatusForbidden, "you are not authorized access to conferences"))
		}
		return next(c)
	}
}

// SetConference is middleware that sets the conference context based on the URL.
func SetConference(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctxt := AmContextFromEchoContext(c)
		conf, err := database.AmGetConferenceByAliasInCommunity(ctxt.Ctx(), ctxt.CurrentCommunity().Id, ctxt.URLParam("confid"))
		if err != nil {
			return AmSendPageData(c, ctxt, "error", err)
		}
		m, lvl, err := conf.Membership(ctxt.Ctx(), ctxt.CurrentUser())
		if err != nil {
			return AmSendPageData(c, ctxt, "error", err)
		}
		myLevel := ctxt.EffectiveLevel()
		if m && lvl > myLevel {
			myLevel = lvl
		}
		ctxt.SetScratch("currentConference", conf)
		ctxt.SetScratch("currentAlias", ctxt.URLParam("confid"))
		ctxt.SetScratch("levelInConference", myLevel)
		return next(c)
	}
}

// SetTopic is middleware that sets the topic context based on the URL.
func SetTopic(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctxt := AmContextFromEchoContext(c)
		conf := ctxt.GetScratch("currentConference").(*database.Conference)

		var topic *database.Topic = nil
		if rawTopic, err := strconv.ParseInt(ctxt.URLParam("topic"), 10, 16); err == nil {
			topic, err = database.AmGetTopicByNumber(ctxt.Ctx(), conf, int16(rawTopic))
		}
		if topic == nil {
			return AmSendPageData(c, ctxt, "error", echo.NewHTTPError(http.StatusNotFound, fmt.Sprintf("topic not found: %s", ctxt.URLParam("topic"))))
		}
		ctxt.SetScratch("currentTopic", topic)
		return next(c)
	}
}

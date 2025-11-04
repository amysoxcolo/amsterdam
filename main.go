/*
 * Amsterdam Web Communities System
 * Copyright (c) 2025 Erbosoft Metaverse Design Solutions, All Rights Reserved
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

// Package main contains the high-level Amsterdam logic.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"git.erbosoft.com/amy/amsterdam/config"
	"git.erbosoft.com/amy/amsterdam/database"
	"git.erbosoft.com/amy/amsterdam/email"
	"git.erbosoft.com/amy/amsterdam/htmlcheck"
	"git.erbosoft.com/amy/amsterdam/ui"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	log "github.com/sirupsen/logrus"
)

// setupEcho creates, configures, and returns a new Echo instance.
func setupEcho() *echo.Echo {
	e := echo.New()
	e.Logger = &EchoLogrusAdapter{}
	e.Renderer = &ui.TemplateRenderer{}
	e.HTTPErrorHandler = AmErrorHandler
	if !config.CommandLine.DebugPanic {
		e.Use(middleware.RecoverWithConfig(middleware.RecoverConfig{
			LogErrorFunc: LogrusPanicLogging,
		}))
	} else {
		log.Warn("WARNING: --debug-panic in effect - DO NOT use this in production!")
	}
	e.Use(LogrusMiddleware)
	e.Use(session.Middleware(ui.SessionStore))

	fn := ui.AmWrap(NotImplPage)
	e.GET("/TODO/*", fn)
	e.POST("/TODO/*", fn)
	e.GET("/img/*", ui.AmWrap(ui.AmServeImage))

	e.GET("/", ui.AmWrap(TopPage))
	e.GET("/about", ui.AmWrap(AboutPage))
	e.GET("/login", ui.AmWrap(LoginForm))
	e.POST("/login", ui.AmWrap(Login))
	e.GET("/logout", ui.AmWrap(Logout))
	e.GET("/newacct", ui.AmWrap(NewAccountUserAgreement))
	e.GET("/newacct2", ui.AmWrap(NewAccountForm))
	e.POST("/newacct2", ui.AmWrap(NewAccount))
	e.GET("/verify", ui.AmWrap(VerifyEmailForm))
	e.POST("/verify", ui.AmWrap(VerifyEMail))
	e.GET("/passrecovery/:uid/:auth", ui.AmWrap(PasswordRecovery))
	e.GET("/profile", ui.AmWrap(EditProfileForm))
	e.POST("/profile", ui.AmWrap(EditProfile))
	e.GET("/profile_photo", ui.AmWrap(ProfilePhotoForm))
	e.POST("/profile_photo", ui.AmWrap(ProfilePhoto))
	e.GET("/find", ui.AmWrap(FindPage))
	e.POST("/find", ui.AmWrap(Find))
	e.GET("/user/:uname", ui.AmWrap(ShowProfile))
	e.POST("/quick_email", ui.AmWrap(QuickEMail))
	e.GET("/sysadmin", ui.AmWrap(SysAdminMenu))
	e.GET("/create_comm", ui.AmWrap(CreateCommunityForm))
	e.POST("/create_comm", ui.AmWrap(CreateCommunity))
	e.GET("/comm/:cid/profile", ui.AmWrap(ShowCommunity))
	e.GET("/comm/:cid/join", ui.AmWrap(JoinCommunity))
	e.POST("/comm/:cid/join", ui.AmWrap(JoinCommunityWithKey))
	e.GET("/comm/:cid/unjoin", ui.AmWrap(UnjoinCommunity))
	e.POST("/comm/:cid/unjoin", ui.AmWrap(UnjoinCommunityConfirm))
	e.GET("/comm/:cid/members", ui.AmWrap(MemberList))
	e.POST("/comm/:cid/members", ui.AmWrap(MemberSearch))
	e.GET("/comm/:cid/admin", ui.AmWrap(CommunityAdminMenu))
	e.GET("/comm/:cid/admin/profile", ui.AmWrap(CommunityProfileForm))
	e.POST("/comm/:cid/admin/profile", ui.AmWrap(EditCommunityProfile))
	e.GET("/comm/:cid/admin/logo", ui.AmWrap(CommunityLogoForm))
	e.POST("/comm/:cid/admin/logo", ui.AmWrap(EditCommunityLogo))
	e.GET("/comm/:cid/conf", ui.AmWrap(Conferences))
	e.GET("/comm/:cid/conf/:confid", ui.AmWrap(Topics))
	e.GET("/comm/:cid/conf/:confid/new_topic", ui.AmWrap(NewTopicForm))
	e.POST("/comm/:cid/conf/:confid/new_topic", ui.AmWrap(NewTopic))

	return e
}

// main is Ye Olde Main Function.
func main() {
	// Configure the system.
	config.SetupConfig()
	closer, err := database.SetupDb()
	if err != nil {
		panic(fmt.Sprintf("Database open failure: %v", err))
	}
	defer closer()
	closer = email.SetupMailSender()
	defer closer()
	htmlcheck.SetupDicts()
	ui.SetupTemplates()
	closer = ui.SetupSessionManager()
	defer closer()
	closer = ui.SetupAmContext()
	defer closer()

	// Set up Echo.
	e := setupEcho()

	// Set up to trap SIGINT and shut down gracefully
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Start server
	go func() {
		if err := e.Start(":1323"); err != nil && err != http.ErrServerClosed {
			e.Logger.Fatalf("shutting down the server: %v", err)
		}
	}()

	// Wait for the interrupt signal and then gracefully shut the server down.
	<-ctx.Done()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		e.Logger.Fatal(err)
	}
}

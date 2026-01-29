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
	"git.erbosoft.com/amy/amsterdam/util"
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
	e.Use(LogrusMiddleware, ui.SessionStoreInjector, ui.ContextCreator)
	e.Use(ui.IPBanTest, ui.CookieLoginTest)

	fn := ui.AmWrap(NotImplPage)
	e.GET("/TODO/*", fn)
	e.POST("/TODO/*", fn)
	e.GET("/img/*", ui.AmServeImage)
	e.GET("/static/*", ui.AmStaticFileHandler())
	e.GET("/go/:postlink", ui.AmWrap(JumpToShortcut))

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
	e.GET("/hotlist", ui.AmWrap(Hotlist))
	e.GET("/sysadmin", ui.AmWrap(SysAdminMenu))
	e.GET("/create_comm", ui.AmWrap(CreateCommunityForm))
	e.POST("/create_comm", ui.AmWrap(CreateCommunity))
	e.POST("/attachment_upload", ui.AmWrap(AttachmentUpload))
	e.GET("/attachment/:post", ui.AmWrap(AttachmentSend))

	// community group
	commGroup := e.Group("/comm/:cid", ui.SetCommunity)
	fn1 := ui.AmWrap(ShowCommunity)
	commGroup.GET("", fn1)
	commGroup.GET("/profile", fn1)
	commGroup.GET("/join", ui.AmWrap(JoinCommunity))
	commGroup.POST("/join", ui.AmWrap(JoinCommunityWithKey))
	commGroup.GET("/unjoin", ui.AmWrap(UnjoinCommunity))
	commGroup.POST("/unjoin", ui.AmWrap(UnjoinCommunityConfirm))
	commGroup.GET("/members", ui.AmWrap(MemberList))
	commGroup.POST("/members", ui.AmWrap(MemberSearch))
	commGroup.GET("/admin", ui.AmWrap(CommunityAdminMenu))
	commGroup.GET("/admin/profile", ui.AmWrap(CommunityProfileForm))
	commGroup.POST("/admin/profile", ui.AmWrap(EditCommunityProfile))
	commGroup.GET("/admin/logo", ui.AmWrap(CommunityLogoForm))
	commGroup.POST("/admin/logo", ui.AmWrap(EditCommunityLogo))

	// conference group
	commGroup.GET("/conf", ui.AmWrap(Conferences), ui.ValidateConference)
	confGroup := commGroup.Group("/conf/:confid", ui.ValidateConference, ui.SetConference)
	confGroup.GET("", ui.AmWrap(Topics))
	confGroup.GET("/new_topic", ui.AmWrap(NewTopicForm))
	confGroup.POST("/new_topic", ui.AmWrap(NewTopic))
	confGroup.GET("/manage", ui.AmWrap(ConfManage))
	confGroup.GET("/hotlist", ui.AmWrap(AddToHotlist))
	confGroup.GET("/r/:topic", ui.AmWrap(ReadPosts), ui.SetTopic)
	confGroup.POST("/r/:topic", ui.AmWrap(PostInTopic), ui.SetTopic)
	opsGroup := confGroup.Group("/op/:topic", ui.SetTopic)
	opsGroup.GET("/hide", ui.AmWrap(HideTopic))
	opsGroup.GET("/hide/:msg", ui.AmWrap(HideMessage))
	opsGroup.GET("/scribble/:msg", ui.AmWrap(ScribbleMessage))
	opsGroup.GET("/nuke/:msg", ui.AmWrap(NukeMessage))
	opsGroup.GET("/manage", ui.AmWrap(TopicManage))
	opsGroup.GET("/rmbozo/:uid", ui.AmWrap(TopicRemoveBozo))

	return e
}

// ampool is the worker pool for one-shot background tasks.
var ampool *util.WorkerPool

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
	closer = ui.SetupAmSessionManager()
	defer closer()
	closer = ui.SetupAmContext()
	defer closer()

	// Set up to trap SIGINT and shut down gracefully
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Set up ampool.
	ampool = util.AmNewPool(ctx, 4, 128)
	go func() {
		<-ctx.Done()
		ampool.Shutdown()
	}()

	// Set up Echo.
	e := setupEcho()

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

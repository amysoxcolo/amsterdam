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
	"syscall"
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

// GetAndPost is used to have functions that respond to both GET and POST on a URI.
var GetAndPost = []string{http.MethodGet, http.MethodPost}

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

	// This is the set of all middleware functions used by the UI, as opposed to other things.
	uiset := []echo.MiddlewareFunc{ui.SessionStoreInjector, ui.ContextCreator, ui.IPBanTest, ui.CookieLoginTest}

	e.RouteNotFound("/*", ui.AmWrap(AmNotFoundHandler), uiset...)
	e.Match(GetAndPost, "/TODO/*", ui.AmWrap(NotImplPage), uiset...)
	e.GET("/img/*", ui.AmServeImage)
	e.GET("/static/*", ui.AmStaticFileHandler())
	e.GET("/go/:postlink", ui.AmWrap(JumpToShortcut))

	e.GET("/", ui.AmWrap(TopPage), uiset...)
	e.GET("/about", ui.AmWrap(AboutPage), uiset...)
	e.GET("/login", ui.AmWrap(LoginForm), uiset...)
	e.POST("/login", ui.AmWrap(Login), uiset...)
	e.GET("/logout", ui.AmWrap(Logout), uiset...)
	e.GET("/newacct", ui.AmWrap(NewAccountUserAgreement), uiset...)
	e.GET("/newacct2", ui.AmWrap(NewAccountForm), uiset...)
	e.POST("/newacct2", ui.AmWrap(NewAccount), uiset...)
	e.GET("/verify", ui.AmWrap(VerifyEmailForm), uiset...)
	e.POST("/verify", ui.AmWrap(VerifyEMail), uiset...)
	e.GET("/passrecovery/:uid/:auth", ui.AmWrap(PasswordRecovery), uiset...)
	e.GET("/profile", ui.AmWrap(EditProfileForm), uiset...)
	e.POST("/profile", ui.AmWrap(EditProfile), uiset...)
	e.GET("/profile_photo", ui.AmWrap(ProfilePhotoForm), uiset...)
	e.POST("/profile_photo", ui.AmWrap(ProfilePhoto), uiset...)
	e.GET("/find", ui.AmWrap(FindPage), uiset...)
	e.POST("/find", ui.AmWrap(Find), uiset...)
	e.GET("/user/:uname", ui.AmWrap(ShowProfile), uiset...)
	e.POST("/quick_email", ui.AmWrap(QuickEMail), uiset...)
	e.GET("/hotlist", ui.AmWrap(Hotlist), uiset...)
	e.GET("/sideboxes", ui.AmWrap(ManageSideboxes), uiset...)
	e.POST("/sideboxes", ui.AmWrap(AddSidebox), uiset...)
	e.GET("/create_comm", ui.AmWrap(CreateCommunityForm), uiset...)
	e.POST("/create_comm", ui.AmWrap(CreateCommunity), uiset...)
	e.GET("/manage_comm", ui.AmWrap(ManageCommunities), uiset...)
	e.POST("/attachment_upload", ui.AmWrap(AttachmentUpload), uiset...)
	e.GET("/attachment/:post", ui.AmWrap(AttachmentSend), uiset...)
	e.POST("/__invite_send", ui.AmWrap(InviteSend), uiset...)
	sysGroup := e.Group("/sysadmin", uiset...)
	sysGroup.GET("", ui.AmWrap(SysAdminMenu))
	sysGroup.GET("/globals", ui.AmWrap(GlobalPropertiesForm))
	sysGroup.POST("/globals", ui.AmWrap(GlobalPropertiesSet))
	sysGroup.Match(GetAndPost, "/users", ui.AmWrap(UserManagementSearch))
	sysGroup.GET("/users/:uname", ui.AmWrap(UserManagementForm))
	sysGroup.POST("/users/:uname", ui.AmWrap(UserManagementSave))
	sysGroup.GET("/users/:uname/photo", ui.AmWrap(AdminUserPhotoForm))
	sysGroup.POST("/users/:uname/photo", ui.AmWrap(AdminUserPhoto))
	sysGroup.GET("/ipban", ui.AmWrap(IPBanList))
	sysGroup.GET("/ipban/add", ui.AmWrap(AddIPBanForm))
	sysGroup.POST("/ipban/add", ui.AmWrap(AddIPBan))
	sysGroup.Match(GetAndPost, "/audit", ui.AmWrap(SystemAudit))

	// community group
	uiset2 := make([]echo.MiddlewareFunc, len(uiset), len(uiset)+1)
	copy(uiset2, uiset)
	commGroup := e.Group("/comm/:cid", append(uiset2, ui.SetCommunity)...)
	fn := ui.AmWrap(ShowCommunity)
	commGroup.GET("", fn)
	commGroup.GET("/profile", fn)
	commGroup.GET("/join", ui.AmWrap(JoinCommunity))
	commGroup.POST("/join", ui.AmWrap(JoinCommunityWithKey))
	commGroup.GET("/unjoin", ui.AmWrap(UnjoinCommunity("prof")))
	commGroup.GET("/unj", ui.AmWrap(UnjoinCommunity("manage")))
	commGroup.POST("/unjoin", ui.AmWrap(UnjoinCommunityConfirm))
	commGroup.GET("/members", ui.AmWrap(MemberList))
	commGroup.POST("/members", ui.AmWrap(MemberSearch))
	commGroup.GET("/invite", ui.AmWrap(InviteToCommunity))
	commGroup.GET("/find", ui.AmWrap(FindPostsPageCommunity))
	commGroup.POST("/find", ui.AmWrap(FindPostsCommunity))
	adminGroup := commGroup.Group("/admin")
	adminGroup.GET("", ui.AmWrap(CommunityAdminMenu))
	adminGroup.GET("/profile", ui.AmWrap(CommunityProfileForm))
	adminGroup.POST("/profile", ui.AmWrap(EditCommunityProfile))
	adminGroup.GET("/logo", ui.AmWrap(CommunityLogoForm))
	adminGroup.POST("/logo", ui.AmWrap(EditCommunityLogo))
	adminGroup.Match(GetAndPost, "/audit", ui.AmWrap(CommunityAudit))
	adminGroup.GET("/category", ui.AmWrap(CommunityCategory))
	adminGroup.Match(GetAndPost, "/members", ui.AmWrap(CommunityMembers))
	adminGroup.GET("/massmail", ui.AmWrap(CommunityEmailForm))
	adminGroup.POST("/massmail", ui.AmWrap(CommunityEmail))

	// conference group
	commGroup.GET("/create_conf", ui.AmWrap(CreateConferenceForm))
	commGroup.POST("/create_conf", ui.AmWrap(CreateConference))
	commGroup.GET("/manage_conf", ui.AmWrap(ManageConferenceList), ui.ValidateConference)
	commGroup.GET("/manage_conf/del/:confid", ui.AmWrap(ManageDeleteConference), ui.ValidateConference, ui.SetConference)
	commGroup.GET("/conf", ui.AmWrap(Conferences), ui.ValidateConference)

	confGroup := commGroup.Group("/conf/:confid", ui.ValidateConference, ui.SetConference)
	confGroup.GET("", ui.AmWrap(Topics))
	confGroup.GET("/new_topic", ui.AmWrap(NewTopicForm))
	confGroup.POST("/new_topic", ui.AmWrap(NewTopic))
	confGroup.GET("/find", ui.AmWrap(FindPostsPageConference))
	confGroup.POST("/find", ui.AmWrap(FindPostsConference))
	confGroup.GET("/manage", ui.AmWrap(ConfManage))
	confGroup.POST("/pseud", ui.AmWrap(SetPseud))
	confGroup.GET("/fixseen", ui.AmWrap(ConfFixseen))
	confGroup.GET("/edit", ui.AmWrap(EditConferenceForm))
	confGroup.POST("/edit", ui.AmWrap(EditConference))
	confGroup.GET("/aliases", ui.AmWrap(ConferenceAliasForm))
	confGroup.POST("/aliases", ui.AmWrap(ConferenceAliasAdd))
	confGroup.Match(GetAndPost, "/members", ui.AmWrap(ConferenceMembers))
	confGroup.GET("/custom", ui.AmWrap(ConfCustomForm))
	confGroup.POST("/custom", ui.AmWrap(ConfCustom))
	confGroup.GET("/activity", ui.AmWrap(ConfReports))
	confGroup.GET("/email", ui.AmWrap(ConferenceEmailForm))
	confGroup.POST("/email", ui.AmWrap(ConferenceEmail))
	confGroup.GET("/export", ui.AmWrap(ConferenceExportForm))
	confGroup.POST("/export", ui.AmWrap(ConferenceExport))
	confGroup.GET("/delete", ui.AmWrap(DeleteConference))
	confGroup.GET("/hotlist", ui.AmWrap(AddToHotlist))
	confGroup.GET("/invite", ui.AmWrap(InviteToConference))
	confGroup.GET("/r/:topic", ui.AmWrap(ReadPosts), ui.SetTopic)
	confGroup.POST("/r/:topic", ui.AmWrap(PostInTopic), ui.SetTopic)

	opsGroup := confGroup.Group("/op/:topic", ui.SetTopic)
	opsGroup.GET("/find", ui.AmWrap(FindPostsPageTopic))
	opsGroup.POST("/find", ui.AmWrap(FindPostsTopic))
	opsGroup.GET("/hide", ui.AmWrap(HideTopic))
	opsGroup.GET("/freeze", ui.AmWrap(FreezeTopic))
	opsGroup.GET("/archive", ui.AmWrap(ArchiveTopic))
	opsGroup.GET("/stick", ui.AmWrap(StickTopic))
	opsGroup.GET("/delete", ui.AmWrap(DeleteTopic))
	opsGroup.GET("/hide/:msg", ui.AmWrap(HideMessage))
	opsGroup.GET("/scribble/:msg", ui.AmWrap(ScribbleMessage))
	opsGroup.GET("/nuke/:msg", ui.AmWrap(NukeMessage))
	opsGroup.GET("/prune/:msg", ui.AmWrap(PruneMessageAttachment))
	opsGroup.GET("/publish/:msg", ui.AmWrap(PublishMessage))
	opsGroup.GET("/move/:msg", ui.AmWrap(MoveMessageForm))
	opsGroup.POST("/move/:msg", ui.AmWrap(MoveMessage))
	opsGroup.GET("/manage", ui.AmWrap(TopicManage))
	opsGroup.GET("/subscribe", ui.AmWrap(TopicSetSubscribe))
	opsGroup.GET("/invite", ui.AmWrap(InviteToTopic))
	opsGroup.GET("/rmbozo/:uid", ui.AmWrap(TopicRemoveBozo))

	return e
}

// ampool is the worker pool for one-shot background tasks.
var ampool *util.WorkerPool

// main is Ye Olde Main Function.
func main() {
	start := time.Now()
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
	closer = ui.SetupUILayer()
	defer closer()

	// Determine my IP address and the admin user.
	myIP, err := util.MyIPAddress()
	if err != nil {
		panic(err)
	}

	// Set up to trap SIGINT/SIGTERM and shut down gracefully
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Set up ampool.
	ampool = util.AmNewPool(ctx, config.GlobalConfig.Tuning.WorkerTasks, config.GlobalConfig.Tuning.Queues.WorkerTasks)
	go func() {
		<-ctx.Done()
		ampool.Shutdown()
	}()

	// Set up Echo.
	e := setupEcho()

	// Audit the startup
	database.AmStoreAudit(database.AmNewAudit(database.AuditStartup, 0, myIP.String(),
		fmt.Sprintf("version=%s", config.AMSTERDAM_VERSION)))
	defer func() {
		// Audit the shutdown
		database.AmStoreAudit(database.AmNewAudit(database.AuditShutdown, 0, myIP.String()))
	}()

	stime := time.Since(start)
	log.Infof("Amsterdam startup sequence completed in %v", stime)

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

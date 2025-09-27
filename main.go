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
	"git.erbosoft.com/amy/amsterdam/ui"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// setupEcho creates, configures, and returns a new Echo instance.
func setupEcho() *echo.Echo {
	e := echo.New()
	e.Logger = &EchoLogrusAdapter{}
	e.Renderer = &ui.TemplateRenderer{}
	e.Use(middleware.Recover())
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
	e.GET("/newacct", ui.AmWrap(NewAccountUserAgreement))
	e.GET("/newacct2", ui.AmWrap(NewAccountForm))

	return e
}

// main is Ye Olde Main Function.
func main() {
	// Configure the system.
	config.SetupConfig()
	err := database.SetupDb()
	if err != nil {
		panic(fmt.Sprintf("Database open failure: %v", err))
	}
	defer database.ClosedownDb()
	ui.SetupTemplates()
	ui.SetupSessionManager()
	ui.SetupLeftMenus()

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

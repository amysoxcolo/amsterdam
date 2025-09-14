/*
 * Amsterdam Web Communities System
 * Copyright (c) 2025 Erbosoft Metaverse Design Solutions, All Rights Reserved
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */
package main

import (
	"time"

	"github.com/labstack/echo/v4"
	log "github.com/sirupsen/logrus"
)

// Custom Logrus middleware
func LogrusMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		start := time.Now()
		err := next(c)
		stop := time.Now()

		req := c.Request()
		res := c.Response()

		log.WithFields(log.Fields{
			"remote_ip":  c.RealIP(),
			"host":       req.Host,
			"method":     req.Method,
			"uri":        req.RequestURI,
			"status":     res.Status,
			"latency_ms": stop.Sub(start).Milliseconds(),
			"user_agent": req.UserAgent(),
		}).Info("handled request")
		return err
	}
}

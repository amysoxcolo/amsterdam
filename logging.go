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
	"io"
	"time"

	"github.com/labstack/echo/v4"
	glog "github.com/labstack/gommon/log"
	log "github.com/sirupsen/logrus"
)

func init() {
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})
	log.SetLevel(log.InfoLevel)
}

func toglog(l log.Level) glog.Lvl {
	switch l {
	case log.DebugLevel:
		return glog.DEBUG
	case log.InfoLevel:
		return glog.INFO
	case log.WarnLevel:
		return glog.WARN
	case log.ErrorLevel:
		return glog.ERROR
	default:
		return glog.OFF
	}
}

func fromglog(l glog.Lvl) log.Level {
	switch l {
	case glog.DEBUG:
		return log.DebugLevel
	case glog.INFO:
		return log.InfoLevel
	case glog.WARN:
		return log.WarnLevel
	case glog.ERROR:
		return log.ErrorLevel
	default:
		return log.PanicLevel
	}
}

// EchoLogrusAdapter implements echo.Logger using logrus
type EchoLogrusAdapter struct{}

func (l *EchoLogrusAdapter) Output() io.Writer                         { return log.StandardLogger().Out }
func (l *EchoLogrusAdapter) SetOutput(w io.Writer)                     { log.SetOutput(w) }
func (l *EchoLogrusAdapter) Prefix() string                            { return "" }
func (l *EchoLogrusAdapter) SetPrefix(p string)                        {}
func (l *EchoLogrusAdapter) Level() glog.Lvl                           { return toglog(log.GetLevel()) }
func (l *EchoLogrusAdapter) SetLevel(lvl glog.Lvl)                     { log.SetLevel(fromglog(lvl)) }
func (l *EchoLogrusAdapter) Print(i ...interface{})                    { log.Print(i...) }
func (l *EchoLogrusAdapter) Printf(format string, args ...interface{}) { log.Printf(format, args...) }
func (l *EchoLogrusAdapter) Printj(j glog.JSON)                        { log.WithFields(log.Fields(j)).Print() }
func (l *EchoLogrusAdapter) Debug(i ...interface{})                    { log.Debug(i...) }
func (l *EchoLogrusAdapter) Debugf(format string, args ...interface{}) { log.Debugf(format, args...) }
func (l *EchoLogrusAdapter) Debugj(j glog.JSON)                        { log.WithFields(log.Fields(j)).Debug() }
func (l *EchoLogrusAdapter) Info(i ...interface{})                     { log.Info(i...) }
func (l *EchoLogrusAdapter) Infof(format string, args ...interface{})  { log.Infof(format, args...) }
func (l *EchoLogrusAdapter) Infoj(j glog.JSON)                         { log.WithFields(log.Fields(j)).Info() }
func (l *EchoLogrusAdapter) Warn(i ...interface{})                     { log.Warn(i...) }
func (l *EchoLogrusAdapter) Warnf(format string, args ...interface{})  { log.Warnf(format, args...) }
func (l *EchoLogrusAdapter) Warnj(j glog.JSON)                         { log.WithFields(log.Fields(j)).Warn() }
func (l *EchoLogrusAdapter) Error(i ...interface{})                    { log.Error(i...) }
func (l *EchoLogrusAdapter) Errorf(format string, args ...interface{}) { log.Errorf(format, args...) }
func (l *EchoLogrusAdapter) Errorj(j glog.JSON)                        { log.WithFields(log.Fields(j)).Error() }
func (l *EchoLogrusAdapter) Fatal(i ...interface{})                    { log.Fatal(i...) }
func (l *EchoLogrusAdapter) Fatalf(format string, args ...interface{}) { log.Fatalf(format, args...) }
func (l *EchoLogrusAdapter) Fatalj(j glog.JSON)                        { log.WithFields(log.Fields(j)).Fatal() }
func (l *EchoLogrusAdapter) Panic(i ...interface{})                    { log.Panic(i...) }
func (l *EchoLogrusAdapter) Panicf(format string, args ...interface{}) { log.Panicf(format, args...) }
func (l *EchoLogrusAdapter) Panicj(j glog.JSON)                        { log.WithFields(log.Fields(j)).Panic() }
func (l *EchoLogrusAdapter) SetHeader(h string)                        {}

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

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

// Package main contains the high-level Amsterdam logic.
package main

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"sync"
	"time"

	"git.erbosoft.com/amy/amsterdam/config"
	"github.com/dustin/go-humanize"
	"github.com/labstack/echo/v5"
	log "github.com/sirupsen/logrus"
)

/*----------------------------------------------------------------------------
 * slog handler that outputs to Logrus
 *----------------------------------------------------------------------------
 */

// slog2logrus converts slog levels to Logrus levels.
var slog2logrus = map[slog.Level]log.Level{
	slog.LevelDebug: log.DebugLevel,
	slog.LevelInfo:  log.InfoLevel,
	slog.LevelWarn:  log.WarnLevel,
	slog.LevelError: log.ErrorLevel,
}

// SlogLogrusHandler implements slog.Handler and routes to Logrus.
type SlogLogrusHandler struct {
	fields      log.Fields // fields defined in this handler
	groupPrefix string     // group prefix
}

// NewSlogLogrusHandler creates a SlogLogrusHandler with base information.
func NewSlogLogrusHandler() *SlogLogrusHandler {
	rc := new(SlogLogrusHandler{
		fields:      make(log.Fields),
		groupPrefix: "",
	})
	return rc
}

// Enabled returns true if the specified log level is handled.
func (h *SlogLogrusHandler) Enabled(ctx context.Context, lvl slog.Level) bool {
	return log.IsLevelEnabled(slog2logrus[lvl])
}

// Handle sends a slog.Record to the log output.
func (h *SlogLogrusHandler) Handle(ctx context.Context, r slog.Record) error {
	flds := make(log.Fields)
	for k, v := range h.fields {
		flds[h.groupPrefix+k] = v
	}
	r.Attrs(func(a slog.Attr) bool {
		flds[h.groupPrefix+a.Key] = a.Value.Any()
		return true
	})
	ntry := log.NewEntry(log.StandardLogger()).WithTime(r.Time).WithFields(flds)
	ntry.Log(slog2logrus[r.Level], r.Message)
	return nil
}

// WithAttrs creates a new Handler from this one, with extra attributes.
func (h *SlogLogrusHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newh := new(SlogLogrusHandler{fields: make(log.Fields)})
	maps.Copy(newh.fields, h.fields)
	for _, a := range attrs {
		newh.fields[a.Key] = a.Value.Any()
	}
	newh.groupPrefix = h.groupPrefix
	return newh
}

// WithGroup creates a new Handler from this one, with an extra group prefix.
func (h *SlogLogrusHandler) WithGroup(name string) slog.Handler {
	newh := new(SlogLogrusHandler{fields: make(log.Fields)})
	maps.Copy(newh.fields, h.fields)
	newh.groupPrefix = h.groupPrefix + name + "."
	return newh
}

/*----------------------------------------------------------------------------
 * Echo middleware adapters
 *----------------------------------------------------------------------------
 */

// LogrusMiddleware installs Logrus logging into the Echo middleware chain.
func LogrusMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		start := time.Now()
		err := next(c)
		stop := time.Now()

		req := c.Request()
		res := c.Response().(*echo.Response)

		log.WithFields(log.Fields{
			"remote_ip":  c.RealIP(),
			"host":       req.Host,
			"method":     req.Method,
			"uri":        req.RequestURI,
			"status":     res.Status,
			"latency_ms": stop.Sub(start).Milliseconds(),
		}).Info("handled request")
		return err
	}
}

/*----------------------------------------------------------------------------
 * Log output file implementation
 *----------------------------------------------------------------------------
 */

// amLogFile represents the log output file.
type amLogFile struct {
	mutex          sync.Mutex     // mutex for this object
	wr             io.WriteCloser // underlying log file
	curSize        int64          // current size of log file
	maxSize        int64          // maximum size of log file
	logPath        string         // log file path
	keep           int            // number of uncompressed files to keep
	keepCompressed int            // number of compressed files to keep
}

// Write (from io.Writer) writes to the log file.
func (lf *amLogFile) Write(p []byte) (int, error) {
	lf.mutex.Lock()
	n, err := lf.wr.Write(p)
	lf.curSize += int64(n)
	lf.mutex.Unlock()
	return n, err
}

// Close (from io.Closer) closes the log file.
func (lf *amLogFile) Close() error {
	lf.mutex.Lock()
	err := lf.wr.Close()
	lf.wr = nil
	lf.mutex.Unlock()
	return err
}

// rotate closes the log file and moves it to a new name, shuffling the previously stored log files by the same amount.
func (lf *amLogFile) rotate() error {
	if lf.keep == 0 && lf.keepCompressed == 0 {
		return nil // degenerate case, keep the log file the same
	}
	// Close existing logfile if it's open.
	reopen := lf.wr != nil
	if reopen {
		lf.wr.Close()
	}
	// First loop: shuffle down all the uncompressed files
	for i := lf.keep; i >= 1; i-- {
		oldpath := fmt.Sprintf("%s.%d", lf.logPath, i)
		_, err := os.Stat(oldpath)
		if err == nil {
			newpath := fmt.Sprintf("%s.%d", lf.logPath, i+1)
			err = os.Rename(oldpath, newpath)
		} else if errors.Is(err, os.ErrNotExist) {
			err = nil
		}
		if err != nil {
			return err
		}
	}
	// Move the original logfile into the 1 slot.
	target := fmt.Sprintf("%s.1", lf.logPath)
	err := os.Rename(lf.logPath, target)
	if err != nil {
		return err
	}
	lastUncompressed := fmt.Sprintf("%s.%d", lf.logPath, lf.keep+1)
	if lf.keepCompressed > 0 {
		// Second loop: shuffle down all the compressed files
		for i := lf.keep + lf.keepCompressed; i >= lf.keep+1; i-- {
			oldpath := fmt.Sprintf("%s.%d.gz", lf.logPath, i)
			_, err := os.Stat(oldpath)
			if err == nil {
				newpath := fmt.Sprintf("%s.%d.gz", lf.logPath, i+1)
				err = os.Rename(oldpath, newpath)
			} else if errors.Is(err, os.ErrNotExist) {
				err = nil
			}
			if err != nil {
				return err
			}
		}
		// Remove the last compressed file, it "fell off the end."
		oldpath := fmt.Sprintf("%s.%d.gz", lf.logPath, lf.keep+lf.keepCompressed+1)
		_, err := os.Stat(oldpath)
		if err == nil {
			os.Remove(oldpath)
		} else if errors.Is(err, os.ErrNotExist) {
			err = nil
		}
		if err != nil {
			return err
		}
	}
	_, err = os.Stat(lastUncompressed)
	if err == nil {
		if lf.keepCompressed > 0 {
			// Compress that last uncompressed file.
			target := fmt.Sprintf("%s.%d.gz", lf.logPath, lf.keep+1)
			var rd io.ReadCloser
			rd, err = os.Open(lastUncompressed)
			if err == nil {
				var xwr io.WriteCloser
				xwr, err = os.OpenFile(target, os.O_WRONLY|os.O_CREATE, 0o600)
				if err == nil {
					wr := gzip.NewWriter(xwr)
					_, err = io.Copy(wr, rd)
					wr.Close()
					xwr.Close()
				}
				rd.Close()
			}
		}
		// Now remove the last uncompressed file (leaving the compressed copy behind, where applicable)
		os.Remove(lastUncompressed)
	} else if errors.Is(err, os.ErrNotExist) {
		err = nil
	}
	if err != nil {
		return err
	}
	// Reopen the (now empty) log file.
	if reopen {
		var err error
		lf.wr, err = os.OpenFile(lf.logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			return err
		}
		lf.curSize = 0
	}
	return nil
}

// tryRotate sees if the log file needs to be rotated and does so.
func (lf *amLogFile) tryRotate() {
	lf.mutex.Lock()
	if lf.curSize >= lf.maxSize {
		err := lf.rotate()
		if err != nil {
			//log.Error("log rotation failed")
		}
	}
	lf.mutex.Unlock()
}

// open opens the log file and sets up the structure for use.
func (lf *amLogFile) open(path string) error {
	lf.mutex.Lock()
	defer lf.mutex.Unlock()
	lf.logPath = path
	lf.wr = nil
	fi, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// The file doesn't exist, so check the directory and make sure it does.
			dirname := filepath.Dir(path)
			fi, err = os.Stat(dirname)
			if err != nil || !fi.IsDir() {
				return os.ErrNotExist
			}
			lf.curSize = 0
		} else {
			return err
		}
	} else {
		lf.curSize = fi.Size()
	}
	if lf.curSize >= lf.maxSize {
		err = lf.rotate()
		if err != nil {
			return err
		}
	}
	lf.wr, err = os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	return err
}

// logScanner is a goroutine that monitors the log file to see when it needs rotating.
func logScanner(ctx context.Context, lf *amLogFile, done chan bool) {
	d, _ := time.ParseDuration("10s")
	t := time.NewTicker(d)
	for {
		select {
		case <-ctx.Done():
			t.Stop()
			done <- true
			return
		case <-t.C:
			lf.tryRotate()
		}
	}
}

// SetupLogging sets up the log file based on the configuration data.
func SetupLogging() func() {
	loglevel, err := log.ParseLevel(config.GlobalComputedConfig.LogLevel)
	if err != nil {
		loglevel = log.ErrorLevel
	}
	if config.GlobalComputedConfig.DebugMode && loglevel != log.TraceLevel {
		loglevel = log.DebugLevel
	}
	var ctx context.Context = nil
	var cancelfunc context.CancelFunc = nil
	var done chan bool
	var logfile io.WriteCloser = nil
	if !config.GlobalComputedConfig.DebugMode && config.GlobalConfig.Logging.LogFile != "" {
		amlog := new(amLogFile)
		maxlog, err := humanize.ParseBytes(config.GlobalConfig.Logging.MaxLogSize)
		if err != nil {
			maxlog = 16 * 1024 * 1024 // default to 16 megabytes
		}
		amlog.maxSize = int64(maxlog)
		amlog.keep = config.GlobalConfig.Logging.KeepLogFiles
		amlog.keepCompressed = config.GlobalConfig.Logging.KeepCompressedLogFiles
		err = amlog.open(config.GlobalConfig.Logging.LogFile)
		if err == nil {
			logfile = amlog
			ctx, cancelfunc = context.WithCancel(context.Background())
			done = make(chan bool)
			go logScanner(ctx, amlog, done)
		}
	}
	if logfile == nil {
		log.SetOutput(os.Stdout)
	} else {
		log.SetOutput(logfile)

	}
	log.SetLevel(loglevel)

	return func() {
		if logfile != nil {
			log.SetOutput(os.Stdout)
			cancelfunc()
			<-done
			logfile.Close()
		}
	}
}

/*----------------------------------------------------------------------------
 * Initialization
 *----------------------------------------------------------------------------
 */

// init sets up the initial configuration for Logrus logging.
func init() {
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})
	log.SetLevel(log.DebugLevel)
}

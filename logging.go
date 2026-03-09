/*
 * Amsterdam Web Communities System
 * Copyright (c) 2025-2026 Erbosoft Metaverse Design Solutions, All Rights Reserved
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

// Package main contains the high-level Amsterdam logic.
package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"git.erbosoft.com/amy/amsterdam/config"
	"github.com/dustin/go-humanize"
	"github.com/labstack/echo/v4"
	glog "github.com/labstack/gommon/log"
	log "github.com/sirupsen/logrus"
)

/*----------------------------------------------------------------------------
 * Gommon-log to logrus adapter
 *----------------------------------------------------------------------------
 */

/* toglog converts a Logrus logging level to a glog one.
 * Parameters:
 *     l - The Logrus log level to be converted.
 * Returns:
 *     The equivalent glog log level.
 */
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

/* fromglog converts a glog logging level to a Logrus one.
 * Parameters:
 *     l - The glog log level to be converted.
 * Returns:
 *     The equivalent Logrus log level.
 */
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

// EchoLogrusAdapter implements echo.Logger using logrus.
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

/*----------------------------------------------------------------------------
 * Echo middleware adapters
 *----------------------------------------------------------------------------
 */

// LogrusMiddleware installs Logrus logging into the Echo middleware chain.
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
		}).Info("handled request")
		return err
	}
}

// LogrusPanicLogging is a log function hooked into the recovery middleware.
func LogrusPanicLogging(c echo.Context, err error, stack []byte) error {
	log.Errorf("[PANIC RECOVERY] %v", err)
	scanner := bufio.NewScanner(bytes.NewReader(stack))
	for scanner.Scan() {
		line := strings.ReplaceAll(scanner.Text(), "\t", "    ")
		log.Error(line)
	}
	return scanner.Err()
}

/*----------------------------------------------------------------------------
 * Log output file implementation
 *----------------------------------------------------------------------------
 */

// amLogFile represents the log output file.
type amLogFile struct {
	mutex          sync.Mutex
	wr             io.WriteCloser
	curSize        int64
	maxSize        int64
	logPath        string
	keep           int
	keepCompressed int
}

// Write (from io.Writer) writes to the log file.
func (lf *amLogFile) Write(p []byte) (int, error) {
	lf.mutex.Lock()
	defer lf.mutex.Unlock()
	n, err := lf.wr.Write(p)
	lf.curSize += int64(n)
	return n, err
}

// Close (from io.Closer) closes the log file.
func (lf *amLogFile) Close() error {
	lf.mutex.Lock()
	defer lf.mutex.Unlock()
	err := lf.wr.Close()
	lf.wr = nil
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
	defer lf.mutex.Unlock()
	if lf.curSize >= lf.maxSize {
		err := lf.rotate()
		if err != nil {
			log.Error("log rotation failed")
		}
	}
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

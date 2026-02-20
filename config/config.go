/*
 * Amsterdam Web Communities System
 * Copyright (c) 2025-2026 Erbosoft Metaverse Design Solutions, All Rights Reserved
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

// Package config contains support for Amsterdam site-wide configuration data.
package config

import (
	_ "embed"
	"errors"
	"fmt"
	"maps"
	"os"
	"regexp"
	"strconv"

	argparse "github.com/alexflint/go-arg"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// AMSTERDAM_VERSION contains the version number of Amsterdam.
const AMSTERDAM_VERSION = "0.0.1"

// AMSTERDAM_COPYRIGHT contains the copyright dates for Amsterdam.
const AMSTERDAM_COPYRIGHT = "2025-2026"

// AmCLI is the command-line interface arguments structure.
type AmCLI struct {
	ConfigFile       string `arg:"-C,--config" help:"Location of the configuration file."`
	DebugPanic       bool   `arg:"--debug-panic" help:"Development Only - disable Echo panic recovery"`
	BuggyAttachments bool   `arg:"--buggy-attachments" help:"Some attachments may be buggy - truncate data if necessary"`
}

// CommandLine is the command-line arguments passed to Amsterdam.
var CommandLine AmCLI

// Description (from argparse.Described) returns the description string for the application.
func (*AmCLI) Description() string {
	return "Amsterdam Web Communities System Server"
}

// Version (from argparse.Versioned) returns the version number string for the application.
func (*AmCLI) Version() string {
	return "Amsterdam " + AMSTERDAM_VERSION
}

// AmConfig holds the configuration of the application as read from YAML.
type AmConfig struct {
	Site struct {
		BaseURL         string `yaml:"baseURL"`
		Title           string `yaml:"title"`
		TopRefresh      int    `yaml:"topRefresh"`
		LoginCookieName string `yaml:"loginCookieName"`
		LoginCookieAge  int    `yaml:"loginCookieAge"`
		SessionExpire   string `yaml:"sessionExpire"`
		UserAgreement   struct {
			Title string `yaml:"title"`
			Text  string `yaml:"text"`
		} `yaml:"userAgreement"`
	} `yaml:"site"`
	Database struct {
		Driver string `yaml:"driver"`
		Dsn    string `yaml:"dsn"`
	} `yaml:"database"`
	Defaults struct {
		Language string `yaml:"language"`
		TimeZone string `yaml:"timezone"`
	} `yaml:"defaults"`
	Email struct {
		Host         string `yaml:"host"`
		Port         int    `yaml:"port"`
		Tls          string `yaml:"tls"`
		AuthType     string `yaml:"authType"`
		User         string `yaml:"user"`
		Password     string `yaml:"password"`
		MailFromAddr string `yaml:"mailFromAddr"`
		MailFromName string `yaml:"mailFromName"`
		Signature    string `yaml:"signature"`
		Disclaimer   string `yaml:"disclaimer"`
	} `yaml:"email"`
	Rendering struct {
		TemplateDir string `yaml:"templatedir"`
		CookieKey   string `yaml:"cookiekey"`
		CountryList struct {
			Prioritize string `yaml:"prioritize"`
		} `yaml:"countryList"`
	} `yaml:"rendering"`
	Posting struct {
		ExternalDictionary string `yaml:"externalDictionary"`
		Uploads            struct {
			MaxSize         string   `yaml:"maxSize"`
			NoCompressTypes []string `yaml:"noCompressTypes"`
		} `yaml:"uploads"`
	} `yaml:"posting"`
	Tuning struct {
		WorkerTasks int `yaml:"workerTasks"`
		Queues      struct {
			AuditWrites    int `yaml:"auditWrites"`
			ContextRecycle int `yaml:"contextRecycle"`
			EmailRecycle   int `yaml:"emailRecycle"`
			EmailSend      int `yaml:"emailSend"`
			IPBans         int `yaml:"ipBans"`
			WorkerTasks    int `yaml:"workerTasks"`
		} `yaml:"queues"`
		Caches struct {
			Communities     int `yaml:"communities"`
			CommunityProps  int `yaml:"communityProps"`
			Conferences     int `yaml:"conferences"`
			ConferenceProps int `yaml:"conferenceProps"`
			ContactInfo     int `yaml:"contactInfo"`
			Members         int `yaml:"members"`
			Menus           int `yaml:"menus"`
			Services        int `yaml:"services"`
			Users           int `yaml:"users"`
			UserProps       int `yaml:"userProps"`
		} `yaml:"caches"`
	} `yaml:"tuning"`
}

type AmConfigComputed struct {
	UploadMaxSize    int32
	UploadNoCompress map[string]bool
}

//go:embed default.yaml
var defaultConfigData []byte

// defaultConfig holds the default configuration data.
var defaultConfig AmConfig

// GlobalConfig holds the global configuration.
var GlobalConfig AmConfig

// GlobalComputedConfig holds the computed values based on GlobalConfig.
var GlobalComputedConfig AmConfigComputed

// init prepares the default configuration for the application.
func init() {
	if err := yaml.Unmarshal(defaultConfigData, &defaultConfig); err != nil {
		panic(err) // can't happen
	}
}

/* overlayString is a helper that takes a loaded or defaulted string and returns it.
 * Parameters:
 *     loaded - The string loaded from a configuration file.
 *     defaulted - The default value of this string.
 * Returns:
 *     loaded if it's not empty, otherwise defaulted.
 */
func overlayString(loaded string, defaulted string) string {
	if loaded == "" {
		return defaulted
	}
	return loaded
}

/* overlayString is a helper that takes a loaded or defaulted string array and returns it. (It merges the two
 * if two different arrays are specified.)
 * Parameters:
 *     loaded - The array loaded from a configuration file.
 *     defaulted - The default value of this array.
 * Returns:
 *     Merged version of the two arrays.
 */
func overlayStringArray(loaded, defaulted []string) []string {
	m := make(map[string]bool)
	for _, s := range defaulted {
		m[s] = true
	}
	for _, s := range loaded {
		m[s] = true
	}
	rc := make([]string, 0, len(m))
	for s := range maps.Keys(m) {
		rc = append(rc, s)
	}
	return rc
}

/* overlayInt is a helper that takes a loaded or defaulted integer and returns it.
 * Parameters:
 *     loaded - The integer loaded from a configuration file.
 *     defaulted - The default value of this integer.
 * Returns:
 *     loaded if it's not empty, otherwise defaulted.
 */
func overlayInt(loaded int, defaulted int) int {
	if loaded != 0 {
		return loaded
	}
	return defaulted
}

/* overlayConfig takes two configuration structures and overlays them to create the third.
 * Parameters:
 *     dest - Points to the destination copnfiguration structure.
 *     loaded - Points to the loaded configuration structure.
 *     defaults - Points to the default configuration structure.
 */
func overlayConfig(dest *AmConfig, loaded *AmConfig, defaults *AmConfig) {
	dest.Site.BaseURL = overlayString(loaded.Site.BaseURL, defaults.Site.BaseURL)
	dest.Site.Title = overlayString(loaded.Site.Title, defaults.Site.Title)
	dest.Site.TopRefresh = overlayInt(loaded.Site.TopRefresh, defaults.Site.TopRefresh)
	dest.Site.LoginCookieName = overlayString(loaded.Site.LoginCookieName, defaults.Site.LoginCookieName)
	dest.Site.LoginCookieAge = overlayInt(loaded.Site.LoginCookieAge, defaults.Site.LoginCookieAge)
	dest.Site.SessionExpire = overlayString(loaded.Site.SessionExpire, defaults.Site.SessionExpire)
	dest.Site.UserAgreement.Title = overlayString(loaded.Site.UserAgreement.Title, defaults.Site.UserAgreement.Title)
	dest.Site.UserAgreement.Text = overlayString(loaded.Site.UserAgreement.Text, defaults.Site.UserAgreement.Text)
	dest.Database.Driver = overlayString(loaded.Database.Driver, defaults.Database.Driver)
	dest.Database.Dsn = overlayString(loaded.Database.Dsn, defaults.Database.Dsn)
	dest.Defaults.Language = overlayString(loaded.Defaults.Language, defaults.Defaults.Language)
	dest.Defaults.TimeZone = overlayString(loaded.Defaults.TimeZone, defaults.Defaults.TimeZone)
	dest.Email.Host = overlayString(loaded.Email.Host, defaults.Email.Host)
	dest.Email.Port = overlayInt(loaded.Email.Port, defaults.Email.Port)
	dest.Email.Tls = overlayString(loaded.Email.Tls, defaults.Email.Tls)
	dest.Email.AuthType = overlayString(loaded.Email.AuthType, defaults.Email.AuthType)
	dest.Email.User = overlayString(loaded.Email.User, defaults.Email.User)
	dest.Email.Password = overlayString(loaded.Email.Password, defaults.Email.Password)
	dest.Email.MailFromAddr = overlayString(loaded.Email.MailFromAddr, defaults.Email.MailFromAddr)
	dest.Email.MailFromName = overlayString(loaded.Email.MailFromName, defaults.Email.MailFromName)
	dest.Email.Signature = overlayString(loaded.Email.Signature, defaults.Email.Signature)
	dest.Email.Disclaimer = overlayString(loaded.Email.Disclaimer, defaults.Email.Disclaimer)
	dest.Rendering.TemplateDir = overlayString(loaded.Rendering.TemplateDir, defaults.Rendering.TemplateDir)
	dest.Rendering.CookieKey = overlayString(loaded.Rendering.CookieKey, defaults.Rendering.CookieKey)
	dest.Rendering.CountryList.Prioritize = overlayString(loaded.Rendering.CountryList.Prioritize, defaults.Rendering.CountryList.Prioritize)
	dest.Posting.ExternalDictionary = overlayString(loaded.Posting.ExternalDictionary, defaults.Posting.ExternalDictionary)
	dest.Posting.Uploads.MaxSize = overlayString(loaded.Posting.Uploads.MaxSize, defaults.Posting.Uploads.MaxSize)
	dest.Posting.Uploads.NoCompressTypes = overlayStringArray(loaded.Posting.Uploads.NoCompressTypes, defaults.Posting.Uploads.NoCompressTypes)
	dest.Tuning.WorkerTasks = overlayInt(loaded.Tuning.WorkerTasks, defaults.Tuning.WorkerTasks)
	dest.Tuning.Queues.AuditWrites = overlayInt(loaded.Tuning.Queues.AuditWrites, defaults.Tuning.Queues.AuditWrites)
	dest.Tuning.Queues.ContextRecycle = overlayInt(loaded.Tuning.Queues.ContextRecycle, defaults.Tuning.Queues.ContextRecycle)
	dest.Tuning.Queues.EmailRecycle = overlayInt(loaded.Tuning.Queues.EmailRecycle, defaults.Tuning.Queues.EmailRecycle)
	dest.Tuning.Queues.EmailSend = overlayInt(loaded.Tuning.Queues.EmailSend, defaults.Tuning.Queues.EmailSend)
	dest.Tuning.Queues.IPBans = overlayInt(loaded.Tuning.Queues.IPBans, defaults.Tuning.Queues.IPBans)
	dest.Tuning.Queues.WorkerTasks = overlayInt(loaded.Tuning.Queues.WorkerTasks, defaults.Tuning.Queues.WorkerTasks)
	dest.Tuning.Caches.Communities = overlayInt(loaded.Tuning.Caches.Communities, defaults.Tuning.Caches.Communities)
	dest.Tuning.Caches.CommunityProps = overlayInt(loaded.Tuning.Caches.CommunityProps, defaults.Tuning.Caches.CommunityProps)
	dest.Tuning.Caches.Conferences = overlayInt(loaded.Tuning.Caches.Conferences, defaults.Tuning.Caches.Conferences)
	dest.Tuning.Caches.ConferenceProps = overlayInt(loaded.Tuning.Caches.ConferenceProps, defaults.Tuning.Caches.ConferenceProps)
	dest.Tuning.Caches.ContactInfo = overlayInt(loaded.Tuning.Caches.ContactInfo, defaults.Tuning.Caches.ContactInfo)
	dest.Tuning.Caches.Members = overlayInt(loaded.Tuning.Caches.Members, defaults.Tuning.Caches.Members)
	dest.Tuning.Caches.Menus = overlayInt(loaded.Tuning.Caches.Menus, defaults.Tuning.Caches.Menus)
	dest.Tuning.Caches.Services = overlayInt(loaded.Tuning.Caches.Services, defaults.Tuning.Caches.Services)
	dest.Tuning.Caches.Users = overlayInt(loaded.Tuning.Caches.Users, defaults.Tuning.Caches.Users)
	dest.Tuning.Caches.UserProps = overlayInt(loaded.Tuning.Caches.UserProps, defaults.Tuning.Caches.UserProps)
}

// parseDataSize converts the data size in bytes, kilobytes, megabytes, or gigabytes to a number value.
func parseDataSize(s string) (int32, error) {
	re, err := regexp.Compile(`^\s*(\d+)\s*([KkMmGg]?)[Bb]?`)
	if err != nil {
		return -1, err
	}
	m := re.FindStringSubmatch(s)
	if m == nil {
		return -1, errors.New("invalid value spacified")
	}
	rc, err := strconv.Atoi(m[1])
	if err != nil {
		return -1, err
	}
	switch m[2] {
	case "k", "K":
		rc *= 1024
	case "m", "M":
		rc *= (1024 * 1024)
	case "g", "G":
		rc *= (1024 * 1024 * 1024)
	}
	return int32(rc), nil
}

// SetupConfig loads the command line arguments, loads the config file, and prepares GlobalConfig.
func SetupConfig() {
	argparse.MustParse(&CommandLine)

	if CommandLine.BuggyAttachments {
		log.Warn("WARNING: --buggy-attachments flag set - NOT recommended for production usage")
	}

	if CommandLine.ConfigFile != "" {
		// load the data and use it to unmarshal the loaded configuration
		data, err := os.ReadFile(CommandLine.ConfigFile)
		if err != nil {
			panic(fmt.Sprintf("unable to load configuration file %s: %v", CommandLine.ConfigFile, err))
		}
		var loadedConfig AmConfig
		if err = yaml.Unmarshal(data, &loadedConfig); err != nil {
			panic(fmt.Sprintf("unable to load configuration file %s: %v", CommandLine.ConfigFile, err))
		}
		overlayConfig(&GlobalConfig, &loadedConfig, &defaultConfig)
	} else {
		GlobalConfig = defaultConfig // just copy over the defaults
	}

	// Compute additional values.
	tmp, err := parseDataSize(GlobalConfig.Posting.Uploads.MaxSize)
	if err != nil {
		panic(err.Error())
	}
	GlobalComputedConfig.UploadMaxSize = tmp
	GlobalComputedConfig.UploadNoCompress = make(map[string]bool)
	for _, s := range GlobalConfig.Posting.Uploads.NoCompressTypes {
		GlobalComputedConfig.UploadNoCompress[s] = true
	}
}

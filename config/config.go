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

// Package config contains support for Amsterdam site-wide configuration data.
package config

import (
	_ "embed"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"reflect"

	"git.erbosoft.com/amy/amsterdam/util"
	argparse "github.com/alexflint/go-arg"
	"github.com/dustin/go-humanize"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// AMSTERDAM_VERSION contains the version number of Amsterdam.
const AMSTERDAM_VERSION = "0.2.0"

// AMSTERDAM_COPYRIGHT contains the copyright dates for Amsterdam.
const AMSTERDAM_COPYRIGHT = "2025-2026"

// CONFIGFILE_NAME is the name of the standard configuration file.
const CONFIGFILE_NAME = "amsterdam.yaml"

// epsilon is used in testing if a float value is 0.
const epsilon = 1e-9

// AmCLI is the command-line interface arguments structure.
type AmCLI struct {
	ConfigFile       string `arg:"-C,--config,env:AMSTERDAM_CONFIG" help:"Location of the configuration file."`
	Debug            bool   `arg:"-D,--debug,env:AMSTERDAM_DEBUG" help:"Force Amsterdam to run in debug mode."`
	LogLevel         string `arg:"-L,--level,--loglevel,env:AMSTERDAM_LOG_LEVEL" help:"Set the log level for the server."`
	Production       bool   `arg:"-P,--production,env:AMSTERDAM_PROD" help:"Force Amsterdam to run in production mode."`
	MailTLS          string `arg:"-S,--mail-tls,env:AMSTERDAM_MAIL_TLS" help:"The TLS setting for mail connections ('none' or 'starttls')"`
	MailUser         string `arg:"-U,--mail-user,env:AMSTERDAM_MAIL_USER" help:"User name to use with the mail server"`
	MailPassword     string `arg:"-W,--mail-password,env:AMSTERDAM_MAIL_PASSWORD" help:"Password to use with the mail server"`
	MailAuthType     string `arg:"-a,--mail-authtype,env:AMSTERDAM_MAIL_AUTHTYPE" help:"The SMTP authentication type ('none' or 'plain')"`
	DatabaseName     string `arg:"-d,--database-name,env:AMSTERDAM_DATABASE_NAME" help:"Database name to use on the database server."`
	Listen           string `arg:"-l,--listen,env:AMSTERDAM_LISTEN" help:"The local address and port for Amsterdam to listen on."`
	MailHost         string `arg:"-m,--mail-host,env:AMSTERDAM_MAIL_HOST" help:"The SMTP mail host to use to send messages."`
	MailPort         int    `arg:"-o,--mail-port,env:AMSTERDAM_MAIL_PORT" help:"The SMTP port to connect to."`
	DatabasePassword string `arg:"-p,--database-password,env:AMSTERDAM_DATABASE_PASSWORD" help:"Password for the database server."`
	DatabaseHost     string `arg:"-t,--database-host,env:AMSTERDAM_DATABASE_HOST" help:"Hostname for the database server."`
	DatabaseUser     string `arg:"-u,--database-user,env:AMSTERDAM_DATABASE_USER" help:"User name for the database server."`
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

// Epilogue (from argparse.Epilogued) returns an epilogue string for the application.
func (*AmCLI) Epilogue() string {
	return fmt.Sprintf("Copyright © %s Erbosoft Metaverse Design Solutions, All Rights Reserved.\nLicensed under MPL 2.0. https://erbosoft.com", AMSTERDAM_COPYRIGHT)
}

// AmConfig holds the configuration of the application as read from YAML.
type AmConfig struct {
	Site struct {
		Production bool   `yaml:"production"`
		Listen     string `yaml:"listen"`
		BaseURL    string `yaml:"baseURL"`
		Title      string `yaml:"title"`
		SiteIcon   struct {
			Path string `yaml:"path"`
			Type string `yaml:"type"`
		} `yaml:"siteIcon"`
		SiteShortcutIcon      string `yaml:"siteShortcutIcon"`
		SiteAppleIcon         string `yaml:"siteAppleIcon"`
		SiteLogo              string `yaml:"siteLogo"`
		TopRefresh            int    `yaml:"topRefresh"`
		LoginCookieName       string `yaml:"loginCookieName"`
		LoginCookieAge        int    `yaml:"loginCookieAge"`
		SessionExpire         string `yaml:"sessionExpire"`
		UserAgreementResource string `yaml:"userAgreementResource"`
		PolicyResource        string `yaml:"policyResource"`
		FooterTemplate        string `yaml:"footerTemplate"`
		DefaultCommunityLogo  string `yaml:"defaultCommunityLogo"`
		DefaultUserPhoto      string `yaml:"defaultUserPhoto"`
		WelcomeTitle          string `yaml:"welcomeTitle"`
		WelcomeMessage        string `yaml:"welcomeMessage"`
		TopPostsTitle         string `yaml:"topPostsTitle"`
		RateLimit             struct {
			Rate          float64 `yaml:"rate"`
			Burst         int     `yaml:"burst"`
			ExpireMinutes int     `yaml:"expireMinutes"`
		} `yaml:"rateLimit"`
	} `yaml:"site"`
	Database struct {
		Driver       string `yaml:"driver"`
		HostName     string `yaml:"hostName"`
		User         string `yaml:"user"`
		Password     string `yaml:"password"`
		DatabaseName string `yaml:"databaseName"`
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
	Logging struct {
		LogFile                string `yaml:"logFile"`
		MaxLogSize             string `yaml:"maxLogSize"`
		KeepLogFiles           int    `yaml:"keepLogFiles"`
		KeepCompressedLogFiles int    `yaml:"keepCompressedLogFiles"`
		LogLevel               string `yaml:"logLevel"`
	} `yaml:"logging"`
	Rendering struct {
		CookieKey   string `yaml:"cookiekey"`
		CountryList struct {
			Prioritize string `yaml:"prioritize"`
		} `yaml:"countryList"`
		VeniceCompatibleImageURLs bool `yaml:"veniceCompatibleImageURLs"`
	} `yaml:"rendering"`
	Resources struct {
		ViewTemplateDir            string `yaml:"viewTemplateDir"`
		DialogTemplateDir          string `yaml:"dialogTemplateDir"`
		EmailTemplateDir           string `yaml:"emailTemplateDir"`
		ExternalContentPath        string `yaml:"externalContentPath"`
		ExternalResourcePath       string `yaml:"externalResourcePath"`
		ExternalMenuDefinitions    string `yaml:"externalMenuDefinitions"`
		ExternalMessageDefinitions string `yaml:"externalMessageDefinitions"`
	} `yaml:"resources"`
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
			Ads             int `yaml:"ads"`
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
	baseDir string // the base directory to evaluate relative paths to.
}

// ExPath expands a path relative to the baseDir (the location of the config file).
func (c *AmConfig) ExPath(path string) string {
	if path == "" || c.baseDir == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(c.baseDir, path)
}

// AmConfigComputed is the configuration values which are "computed" based only on values in AmConfig.
type AmConfigComputed struct {
	DebugMode        bool            // are we in debug mode?
	LogLevel         string          // the logging level
	Listen           string          // listen address
	DatabaseDriver   string          // name of database driver
	DatabaseHost     string          // hostname for database
	DatabaseUser     string          // user name for database
	DatabasePassword string          // password for database
	DatabaseName     string          // database name
	MailHost         string          // SMTP host
	MailPort         int             // SMTP port
	MailTLS          string          // SMTP TLS setting
	MailAuthType     string          // SMTP auth type
	MailUser         string          // SMTP user name
	MailPassword     string          // SMTP password
	UploadMaxSize    int32           // maximum upload size in bytes
	UploadNoCompress map[string]bool // which upload types are not compressed?
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

// locateConfigFile locates and opens the Amsterdam configuration file, if it exists.
func locateConfigFile() (string, *os.File) {
	// first, check the one on the command line (or in the environment)
	if CommandLine.ConfigFile != "" {
		f, err := os.Open(CommandLine.ConfigFile)
		if err == nil {
			return CommandLine.ConfigFile, f
		}
	}
	// now, check the OS-specific configuration file directories
	dirs := configFileDirs()
	for _, d := range dirs {
		p := filepath.Join(d, CONFIGFILE_NAME)
		f, err := os.Open(p)
		if err == nil {
			return p, f
		}
	}
	// finally, punt and just use the defaults
	return "", nil
}

// overlayStructValue overlays the "loaded" and "defaults" structure onto the "dest" structure. All parameters are AmConfig structures.
func overlayStructValue(dest, loaded, defaults reflect.Value) {
	typ := dest.Type()
	for i := 0; i < dest.NumField(); i++ {
		structField := typ.Field(i)
		if !structField.IsExported() {
			continue
		}
		fldDest := dest.Field(i)
		fldLoaded := loaded.Field(i)
		fldDefaults := defaults.Field(i)
		if fldDest.Kind() == reflect.Struct {
			// nested struct - call recursively
			overlayStructValue(fldDest, fldLoaded, fldDefaults)
		} else if fldDest.Kind() == reflect.String {
			// string field handling
			s := fldLoaded.Interface().(string)
			if s == "" {
				fldDest.Set(fldDefaults)
			} else {
				fldDest.Set(fldLoaded)
			}
		} else if fldDest.Kind() == reflect.Array || fldDest.Kind() == reflect.Slice {
			// array of strings - merge the two arrays
			m := make(map[string]bool)
			rc := make([]string, 0, fldDefaults.Len()+fldLoaded.Len())
			for i := 0; i < fldDefaults.Len(); i++ {
				m[fldDefaults.Index(i).String()] = true
				rc = append(rc, fldDefaults.Index(i).String())
			}
			for i := 0; i < fldLoaded.Len(); i++ {
				if !m[fldLoaded.Index(i).String()] {
					m[fldLoaded.Index(i).String()] = true
					rc = append(rc, fldLoaded.Index(i).String())
				}
			}
			fldDest.Set(reflect.ValueOf(rc))
		} else if fldDest.Kind() == reflect.Bool {
			// just "or" the boolean values together
			b1 := fldDefaults.Bool()
			b2 := fldLoaded.Bool()
			fldDest.SetBool(b1 || b2)
		} else if fldDest.CanInt() {
			// int field handling
			n := fldLoaded.Int()
			if n == 0 {
				fldDest.Set(fldDefaults)
			} else {
				fldDest.Set(fldLoaded)
			}
		} else if fldDest.CanFloat() {
			// float field handling
			n := fldLoaded.Float()
			if math.Abs(n) <= epsilon {
				fldDest.Set(fldDefaults)
			} else {
				fldDest.Set(fldLoaded)
			}
		} else {
			// if we see this message, this function needs more work
			log.Errorf("*** unable to deal with field %s of type %s", structField.Name, typ.Name())
		}
	}
}

// AmOpenExternalContentPath opens the "external content path" specified in the configuration as a root filesystem.
func AmOpenExternalContentPath() (fs.FS, error) {
	path := GlobalConfig.ExPath(GlobalConfig.Resources.ExternalContentPath)
	if path == "" {
		return nil, nil
	}
	finfo, err := os.Stat(path)
	if err != nil {
		log.Errorf("external content path \"%s\" is not valid, ignored (%v)", path, err)
		return nil, nil
	}
	if !finfo.IsDir() {
		log.Errorf("external content path \"%s\" is not a directory, ignored", path)
		return nil, nil
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, err
	}
	return root.FS(), nil
}

// SetupConfig loads the command line arguments, loads the config file, and prepares GlobalConfig.
func SetupConfig() {
	argparse.MustParse(&CommandLine)

	if CommandLine.BuggyAttachments {
		log.Warn("WARNING: --buggy-attachments flag set - NOT recommended for production usage")
	}

	// Locate and read the Amsterdam configuration file.
	name, file := locateConfigFile()
	if file != nil {
		log.Infof("SetupConfig(): using config file %s", name)
		data, err := io.ReadAll(file)
		file.Close()
		if err != nil {
			panic(fmt.Sprintf("unable to load configuration file %s: %v", name, err))
		}
		var loadedConfig AmConfig
		if err = yaml.Unmarshal(data, &loadedConfig); err != nil {
			panic(fmt.Sprintf("unable to load configuration file %s: %v", name, err))
		}
		overlayStructValue(reflect.ValueOf(&GlobalConfig).Elem(), reflect.ValueOf(&loadedConfig).Elem(), reflect.ValueOf(&defaultConfig).Elem())
		GlobalConfig.baseDir = filepath.Dir(name)
	} else {
		log.Info("SetupConfig(): using default configs only")
		GlobalConfig = defaultConfig // just copy over the defaults
		GlobalConfig.baseDir = ""
	}

	// Compute additional values.
	if CommandLine.Debug {
		GlobalComputedConfig.DebugMode = true
	} else if CommandLine.Production {
		GlobalComputedConfig.DebugMode = false
	} else {
		GlobalComputedConfig.DebugMode = !GlobalConfig.Site.Production
	}
	GlobalComputedConfig.LogLevel = util.IIF(CommandLine.LogLevel != "", CommandLine.LogLevel, GlobalConfig.Logging.LogLevel)
	GlobalComputedConfig.Listen = util.IIF(CommandLine.Listen != "", CommandLine.Listen, GlobalConfig.Site.Listen)
	GlobalComputedConfig.DatabaseDriver = GlobalConfig.Database.Driver // FUTURE: allow configuration
	GlobalComputedConfig.DatabaseHost = util.IIF(CommandLine.DatabaseHost != "", CommandLine.DatabaseHost, GlobalConfig.Database.HostName)
	GlobalComputedConfig.DatabaseUser = util.IIF(CommandLine.DatabaseUser != "", CommandLine.DatabaseUser, GlobalConfig.Database.User)
	GlobalComputedConfig.DatabasePassword = util.IIF(CommandLine.DatabasePassword != "", CommandLine.DatabasePassword, GlobalConfig.Database.Password)
	GlobalComputedConfig.DatabaseName = util.IIF(CommandLine.DatabaseName != "", CommandLine.DatabaseName, GlobalConfig.Database.DatabaseName)
	GlobalComputedConfig.MailHost = util.IIF(CommandLine.MailHost != "", CommandLine.MailHost, GlobalConfig.Email.Host)
	GlobalComputedConfig.MailPort = util.IIF(CommandLine.MailPort != 0, CommandLine.MailPort, GlobalConfig.Email.Port)
	GlobalComputedConfig.MailTLS = util.IIF(CommandLine.MailTLS != "", CommandLine.MailTLS, GlobalConfig.Email.Tls)
	GlobalComputedConfig.MailAuthType = util.IIF(CommandLine.MailAuthType != "", CommandLine.MailAuthType, GlobalConfig.Email.AuthType)
	GlobalComputedConfig.MailUser = util.IIF(CommandLine.MailUser != "", CommandLine.MailUser, GlobalConfig.Email.User)
	GlobalComputedConfig.MailPassword = util.IIF(CommandLine.MailPassword != "", CommandLine.MailPassword, GlobalConfig.Email.Password)
	tmp, err := humanize.ParseBytes(GlobalConfig.Posting.Uploads.MaxSize)
	if err != nil {
		panic(err.Error())
	}
	GlobalComputedConfig.UploadMaxSize = int32(tmp)
	GlobalComputedConfig.UploadNoCompress = make(map[string]bool)
	for _, s := range GlobalConfig.Posting.Uploads.NoCompressTypes {
		GlobalComputedConfig.UploadNoCompress[s] = true
	}
}

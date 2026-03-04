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
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
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

// CONFIGFILE_NAME is the name of the standard configuration file.
const CONFIGFILE_NAME = "amsterdam.yaml"

// AmCLI is the command-line interface arguments structure.
type AmCLI struct {
	ConfigFile       string `arg:"-C,--config,env:AMSTERDAM_CONFIG" help:"Location of the configuration file."`
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
		CookieKey   string `yaml:"cookiekey"`
		CountryList struct {
			Prioritize string `yaml:"prioritize"`
		} `yaml:"countryList"`
		VeniceCompatibleImageURLs bool `yaml:"veniceCompatibleImageURLs"`
	} `yaml:"rendering"`
	Resources struct {
		ViewTemplateDir     string `yaml:"viewTemplateDir"`
		ExternalContentPath string `yaml:"externalContentPath"`
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
}

// AmConfigComputed is the configuration values which are "computed" based only on values in AmConfig.
type AmConfigComputed struct {
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
			for i := 0; i < fldDefaults.Len(); i++ {
				m[fldDefaults.Index(i).String()] = true
			}
			for i := 0; i < fldLoaded.Len(); i++ {
				m[fldLoaded.Index(i).String()] = true
			}
			rc := make([]string, 0, len(m))
			for s := range m {
				rc = append(rc, s)
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
		} else {
			// if we see this message, this function needs more work
			errField := typ.Field(i)
			log.Errorf("*** unable to deal with field %s of type %s", errField.Name, typ.Name())
		}
	}
}

// parseDataSize converts the data size in bytes, kilobytes, megabytes, or gigabytes to a number value.
func parseDataSize(s string) (int32, error) {
	re, err := regexp.Compile(`^\s*(\d+)\s*([KkMmGg]?)[Bb]?`)
	if err != nil {
		return -1, err
	}
	m := re.FindStringSubmatch(s)
	if m == nil {
		return -1, errors.New("invalid value specified")
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

// AmOpenExternalContentPath opens the "external content path" specified in the configuration as a root filesystem.
func AmOpenExternalContentPath() (fs.FS, error) {
	if GlobalConfig.Resources.ExternalContentPath == "" {
		return nil, nil
	}
	finfo, err := os.Stat(GlobalConfig.Resources.ExternalContentPath)
	if err != nil {
		log.Errorf("external content path \"%s\" is not valid, ignored (%v)", GlobalConfig.Resources.ExternalContentPath, err)
		return nil, nil
	}
	if !finfo.IsDir() {
		log.Errorf("external content path \"%s\" is not a directory, ignored", GlobalConfig.Resources.ExternalContentPath)
		return nil, nil
	}
	root, err := os.OpenRoot(GlobalConfig.Resources.ExternalContentPath)
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
	} else {
		log.Info("SetupConfig(): using default configs only")
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

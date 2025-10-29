/*
 * Amsterdam Web Communities System
 * Copyright (c) 2025 Erbosoft Metaverse Design Solutions, All Rights Reserved
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */
// The htmlcheck package contains the HTML Checker.
package htmlcheck

import (
	_ "embed"
	"math"
	"strings"

	"gopkg.in/yaml.v3"
)

// EmoticonDef is a single emoticon definition.
type EmoticonDef struct {
	Name     string   `yaml:"name"`
	Patterns []string `yaml:"patterns"`
	Replace  string   `yaml:"replace"`
}

// EmoticonConfig is the YAML configuration of the emoticons.
type EmoticonConfig struct {
	PrefixChars string        `yaml:"prefixChars"`
	Emoticons   []EmoticonDef `yaml:"emoticons"`
}

// emoticonRewriter is the implementation of rewriter in this file
type emoticonRewriter struct {
	config      *EmoticonConfig
	prefixChars []byte
	emos        map[string]*EmoticonDef
	patterns    map[string]string
	minLength   int
}

//go:embed emoticons.yaml
var rawEmoConfig []byte

// EmoticonRewriter is the singleton instance of the emoticon rewriter.
var EmoticonRewriter rewriter

// init loads the configuration and creates the singleton instance.
func init() {
	var cfg EmoticonConfig
	if err := yaml.Unmarshal(rawEmoConfig, &cfg); err != nil {
		panic(err)
	}
	rw := emoticonRewriter{
		config:      &cfg,
		prefixChars: []byte(cfg.PrefixChars),
		emos:        make(map[string]*EmoticonDef),
		patterns:    make(map[string]string),
		minLength:   math.MaxInt,
	}
	for i, def := range rw.config.Emoticons {
		rw.emos[def.Name] = &(rw.config.Emoticons[i])
		for _, p := range def.Patterns {
			f := false
			for k := range rw.prefixChars {
				if p[0] == rw.prefixChars[k] {
					f = true
					break
				}
			}
			if f {
				rw.patterns[p] = def.Name
				rw.minLength = min(rw.minLength, len(p))
			}
		}
	}
	EmoticonRewriter = &rw
}

// Name returns the rewriter's name.
func (rw *emoticonRewriter) Name() string {
	return "emoticon"
}

/* Rewrite rewrites the given string data and adds markup before and after if needed.
 * Parameters:
 *     data - The data to be rewritten.
 *     svc - Services interface we can use.
 * Returns:
 *     Pointer to markup data, or nil.
 */
func (rw *emoticonRewriter) Rewrite(data string, svc rewriterServices) *markupData {
	pos := math.MaxInt
	for _, c := range rw.prefixChars {
		foo := strings.IndexByte(data, c)
		if foo >= 0 {
			pos = min(pos, foo)
		}
	}
	if pos == math.MaxInt {
		return nil
	}
	didReplace := false
	var output strings.Builder
	work := data
	for pos != math.MaxInt {
		if pos > 0 {
			output.WriteString(work[:pos])
			work = work[pos:]
		}
		looking := true
		if len(work) >= rw.minLength {
			for k, v := range rw.patterns {
				if strings.HasPrefix(work, k) {
					looking = false
					output.WriteString(rw.emos[v].Replace)
					work = work[len(k):]
					didReplace = true
					break
				}
			}
		}
		if looking {
			output.WriteString(work[:1])
			work = work[1:]
		}
		pos = math.MaxInt
		for _, c := range rw.prefixChars {
			foo := strings.IndexByte(work, c)
			if foo >= 0 {
				pos = min(pos, foo)
			}
		}
	}
	if !didReplace {
		return nil
	}
	output.WriteString(work)
	return &markupData{beginMarkup: "", text: output.String(), endMarkup: "", rescan: true}
}

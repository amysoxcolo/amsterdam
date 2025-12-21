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
	"context"
	_ "embed"
	"math"
	"regexp"
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
	emos        map[string]*EmoticonDef
}

// emoticonRewriter is the implementation of rewriter in this file.
type emoticonRewriter struct {
	config      *EmoticonConfig
	prefixChars []byte
	patterns    map[string]string
	minLength   int
}

//go:embed emoticons.yaml
var rawEmoConfig []byte

// init loads the configuration and registers the rewriters.
func init() {
	var cfg EmoticonConfig
	if err := yaml.Unmarshal(rawEmoConfig, &cfg); err != nil {
		panic(err)
	}
	cfg.emos = make(map[string]*EmoticonDef)
	for i, def := range cfg.Emoticons {
		cfg.emos[def.Name] = &(cfg.Emoticons[i])
	}
	rw := emoticonRewriter{
		config:      &cfg,
		prefixChars: []byte(cfg.PrefixChars),
		patterns:    make(map[string]string),
		minLength:   math.MaxInt,
	}
	for _, def := range rw.config.Emoticons {
		for _, p := range def.Patterns {
			f := false
			for i := range rw.prefixChars {
				if p[0] == rw.prefixChars[i] {
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
	rewriterRegistry[rw.Name()] = &rw

	rw2 := emoticonTagRewriter{
		config: &cfg,
		re:     regexp.MustCompile(`^ei:\s*(\w+)(\s*/)?\s*$`),
	}
	rewriterRegistry[rw2.Name()] = &rw2
}

// Name returns the rewriter's name.
func (rw *emoticonRewriter) Name() string {
	return "emoticon"
}

/* Rewrite rewrites the given string data and adds markup before and after if needed.
 * Parameters:
 *     ctx - Standard Go context value.
 *     data - The data to be rewritten.
 *     svc - Services interface we can use.
 * Returns:
 *     Pointer to markup data, or nil.
 */
func (rw *emoticonRewriter) Rewrite(ctx context.Context, data string, svc rewriterServices) *markupData {
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
					output.WriteString(rw.config.emos[v].Replace)
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
	return &markupData{
		beginMarkup: "",
		text:        output.String(),
		endMarkup:   "",
		rescan:      true,
	}
}

// emoticonTagRewriter rewrites emoticon tags.
type emoticonTagRewriter struct {
	config *EmoticonConfig
	re     *regexp.Regexp
}

// Name returns the rewriter's name.
func (rw *emoticonTagRewriter) Name() string {
	return "emoticon_tag"
}

/* Rewrite rewrites the given string data and adds markup before and after if needed.
 * Parameters:
 *     ctx - Standard Go context value.
 *     data - The data to be rewritten.
 *     svc - Services interface we can use.
 * Returns:
 *     Pointer to markup data, or nil.
 */
func (rw *emoticonTagRewriter) Rewrite(ctx context.Context, data string, svc rewriterServices) *markupData {
	m := rw.re.FindStringSubmatch(data)
	if m == nil {
		return nil
	}
	d, ok := rw.config.emos[m[1]]
	if !ok {
		return nil
	}
	return &markupData{
		beginMarkup: "",
		text:        d.Replace,
		endMarkup:   "",
		rescan:      false,
	}
}

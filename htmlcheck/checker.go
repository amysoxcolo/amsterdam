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

// HTMLChecker is a component that checks HTML and reformats it as needed.
type HTMLChecker interface {
	Append(string) error
	Finish() error
	Reset()
	Value() (string, error)
	Length() (int, error)
	Lines() (int, error)
	Counter(string) (int, error)
	Context() map[string]any
	ExternalRefs() ([]any, error)
	InternalRefs() ([]any, error)
}

// var NotYetFinished = errors.New("the HTML checker has not yet been finished")

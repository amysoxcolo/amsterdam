/*
 * Amsterdam Web Communities System
 * Copyright (c) 2025 Erbosoft Metaverse Design Solutions, All Rights Reserved
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

// Package util contains utility definitions.
package util

import "sync"

// freeListElem is an element of the free list.
type freeListElem[T any] struct {
	next *freeListElem[T]
	prev *freeListElem[T]
	data *T
}

// FreeList defines a free list.
type FreeList[T any] struct {
	New     func() *T
	mutex   sync.Mutex
	listptr *freeListElem[T]
}

// Put adds a value to the free list.
func (l *FreeList[T]) Put(value *T) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	ne := freeListElem[T]{data: value}
	if l.listptr == nil {
		ne.next = &ne
		ne.prev = &ne
		l.listptr = &ne
	} else {
		ne.next = l.listptr
		ne.prev = l.listptr.prev
		ne.next.prev = &ne
		ne.prev.next = &ne
	}
}

// Get removes a value from the free list. If there are no values and New is specified, is calls that.
func (l *FreeList[T]) Get() *T {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	var rc *T = nil
	if l.listptr == nil {
		if l.New != nil {
			rc = l.New()
		}
	} else {
		elt := l.listptr
		rc = elt.data
		l.listptr = elt.next
		if l.listptr == elt {
			l.listptr = nil
		} else {
			elt.prev.next = elt.next
			elt.next.prev = elt.prev
		}
	}
	return rc
}

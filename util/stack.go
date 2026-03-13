/*
 * Amsterdam Web Communities System
 * Copyright (c) 2025 Erbosoft Metaverse Design Solutions, All Rights Reserved
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * SPDX-License-Identifier: MPL-2.0
 */

// Package util contains utility definitions.
package util

// Stack[T] is a simple generic array-based stack implementation.
type Stack[T comparable] struct {
	elements []T
}

// IsEmpty returns true if the stack is empty.
func (stk *Stack[T]) IsEmpty() bool {
	return len(stk.elements) == 0
}

// Push adds a value to the top of the stack.
func (stk *Stack[T]) Push(data T) {
	stk.elements = append(stk.elements, data)
}

// Pop removes and returns a value from the top of the stack.
func (stk *Stack[T]) Pop() (T, bool) {
	if stk.IsEmpty() {
		return *new(T), false
	}
	topElement := stk.elements[len(stk.elements)-1]
	stk.elements = stk.elements[:len(stk.elements)-1]
	return topElement, true
}

// Peek returns the current value on the top of the stack.
func (stk *Stack[T]) Peek() (T, bool) {
	if stk.IsEmpty() {
		return *new(T), false
	}
	return stk.elements[len(stk.elements)-1], true
}

// RemoveMostRecent looks for the most recent particular data element on the stack, and removes that.
func (stk *Stack[T]) RemoveMostRecent(data T) bool {
	i := len(stk.elements) - 1
	for i >= 0 {
		if stk.elements[i] == data {
			if i == 0 {
				stk.elements = stk.elements[1:]
			} else if (i + 1) == len(stk.elements) {
				stk.elements = stk.elements[:i]
			} else {
				stk.elements = append(stk.elements[:i], stk.elements[i+1:]...)
			}
			return true
		}
		i--
	}
	return false
}

// Clear clears out the stack.
func (stk *Stack[T]) Clear() {
	stk.elements = make([]T, 0)
}

// NewStack creates and returns a new stack.
func NewStack[T comparable]() *Stack[T] {
	return &Stack[T]{
		elements: make([]T, 0),
	}
}

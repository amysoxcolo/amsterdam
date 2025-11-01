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

// Stack[T] is a simple generic array-based stack implementation.
type Stack[T any] struct {
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

// NewStack creates and returns a new stack.
func NewStack[T any]() *Stack[T] {
	return &Stack[T]{
		elements: make([]T, 0),
	}
}

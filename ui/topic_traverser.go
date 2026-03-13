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

// Package ui holds the support for the Amsterdam user interface, wrapping Echo and Jet templates.
package ui

import (
	"sync"

	"git.erbosoft.com/amy/amsterdam/database"
	"github.com/bits-and-blooms/bitset"
)

// TopicTraverser is the data structure that allows us to navigate to "next" topics.
type TopicTraverser interface {
	FirstTopic() int16
	NextTopic(int16) int16
	ClearTopic(int16)
	UnclearTopic(int16)
}

// topicTraverser is the internal data structure that implements TopicTraverser.
type topicTraverser struct {
	lock   sync.RWMutex
	topics []int16
	active *bitset.BitSet
}

// NewTopicTraverser creates the traverser data structure from the topic listing.
func NewTopicTraverser(topics []*database.TopicSummary) TopicTraverser {
	trav := topicTraverser{
		topics: make([]int16, 0, len(topics)),
		active: bitset.New(uint(len(topics))),
	}
	p := 0
	for _, t := range topics {
		if t.Unread > 0 {
			trav.topics = append(trav.topics, t.Number)
			trav.active.Set(uint(p))
			p++
		}
	}
	return &trav
}

// FirstTopic returns the first unread topic number in the traverser.
func (trav *topicTraverser) FirstTopic() int16 {
	trav.lock.RLock()
	defer trav.lock.RUnlock()
	i, b := trav.active.NextSet(0)
	if b {
		return trav.topics[i]
	}
	return -1
}

// NextTopic returns the unread topic number in the traverser after the specified one.
func (trav *topicTraverser) NextTopic(cur int16) int16 {
	trav.lock.RLock()
	defer trav.lock.RUnlock()
	seeking := false
	for i, v := range trav.topics {
		if v == cur {
			seeking = true
		} else if seeking && trav.active.Test(uint(i)) {
			return v
		}
	}
	return trav.FirstTopic() // look from the beginning
}

// ClearTopic clears the specified topic number from the traverser.
func (trav *topicTraverser) ClearTopic(num int16) {
	trav.lock.Lock()
	defer trav.lock.Unlock()
	for i, v := range trav.topics {
		if v == num {
			trav.active.Clear(uint(i))
			return
		}
	}
}

// UnclearTopic restores the specified topic number to the traverser.
func (trav *topicTraverser) UnclearTopic(num int16) {
	trav.lock.Lock()
	defer trav.lock.Unlock()
	for i, v := range trav.topics {
		if v == num {
			trav.active.Set(uint(i))
			return
		}
	}
}

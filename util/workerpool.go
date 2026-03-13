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

import (
	"context"
	"sync"

	log "github.com/sirupsen/logrus"
)

// Task is a function that can be submitted as a one-shot task.
type Task func(ctx context.Context)

// WorkerPool is a pool that can be used to submit one-shot background tasks.
type WorkerPool struct {
	ctx    context.Context    // context
	cancel context.CancelFunc // cancellation function
	tasks  chan Task          // our task queue
	wg     sync.WaitGroup     // wait group for shutdown
}

/* AmNewPool creates a new WorkerPool.
 * Parameters:
 *     parent - The parent context for the worker pool.
 *     workers - The number of worker goroutines to spawn.
 *     queueSize - The size of the task queue.
 * Returns:
 *     Pointer to the new WorkerPool.
 */
func AmNewPool(parent context.Context, workers, queueSize int) *WorkerPool {
	ctx, cancel := context.WithCancel(parent)
	p := WorkerPool{
		ctx:    ctx,
		cancel: cancel,
		tasks:  make(chan Task, queueSize),
	}
	for i := range workers {
		p.wg.Go(func() {
			p.worker(i)
		})
	}
	return &p
}

// worker is the worker goroutine for a pool.
func (p *WorkerPool) worker(id int) {
	for {
		select {
		case <-p.ctx.Done():
			return
		case task, ok := <-p.tasks:
			if !ok {
				return
			}
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Errorf("worker %d panic: %v", id, r)
					}
				}()
				task(p.ctx)
			}()
		}
	}
}

// Submit queues a task for the worker pool.
func (p *WorkerPool) Submit(task Task) bool {
	select {
	case p.tasks <- task:
		return true
	case <-p.ctx.Done():
		return false
	default:
		// queue is full
		return false
	}
}

// Shutdown shuts down the worker pool.
func (p *WorkerPool) Shutdown() {
	p.cancel()
	close(p.tasks)
	p.wg.Wait()
}

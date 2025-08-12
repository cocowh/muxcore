// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pool

import (
	"context"
	"github.com/cocowh/muxcore/pkg/logger"
	"sync"
)

// GoroutinePool represents a pool of goroutines that can execute tasks concurrently.
type GoroutinePool struct {
	tasks   chan func()
	workers int
	wg      sync.WaitGroup
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewGoroutinePool creates a new GoroutinePool with the given number of workers and queue size.
func NewGoroutinePool(workers int, queueSize int) *GoroutinePool {
	if workers <= 0 {
		workers = 1
	}
	if queueSize <= 0 {
		queueSize = 100
	}

	ctx, cancel := context.WithCancel(context.Background())
	pool := &GoroutinePool{
		tasks:   make(chan func(), queueSize),
		workers: workers,
		ctx:     ctx,
		cancel:  cancel,
	}

	// start workers goroutine
	pool.wg.Add(workers)
	for i := 0; i < workers; i++ {
		go pool.worker()
	}

	logger.Infof("Created goroutine pool with %d workers and queue size %d", workers, queueSize)
	return pool
}

// worker is a worker goroutine that processes tasks from the channel
func (p *GoroutinePool) worker() {
	defer p.wg.Done()
	for {
		select {
		case task := <-p.tasks:
			task()
		case <-p.ctx.Done():
			logger.Debug("Goroutine pool worker exiting")
			return
		}
	}
}

// Submit submits a task to the goroutine pool
func (p *GoroutinePool) Submit(task func()) {
	select {
	case p.tasks <- task:
	case <-p.ctx.Done():
		logger.Warn("Cannot submit task to closed goroutine pool")
	default:
		logger.Debug("Goroutine pool queue full, creating temporary worker")
		go task()
	}
}

// Shutdown closes the goroutine pool and waits for all tasks to complete
func (p *GoroutinePool) Shutdown() {
	p.cancel()
	p.wg.Wait()
	logger.Info("Goroutine pool shutdown completed")
}

// GetQueueSize returns the current size of the task queue
func (p *GoroutinePool) GetQueueSize() int {
	return len(p.tasks)
}

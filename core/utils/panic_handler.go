// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package utils

import (
	"runtime/debug"
	"sync"

	"github.com/cocowh/muxcore/pkg/logger"
)

var (
	once         sync.Once
	PanicHandler PanicHandlerFunc
)

func init() {
	PanicHandler = func(f func()) {
		if err := recover(); err != nil {
			logger.Errorf("recover panic. error:%v, stack: %s", err, debug.Stack())
			if f != nil {
				f()
			}
		}
	}
}

type PanicHandlerFunc func(f func())

func SetPanicHandler(f PanicHandlerFunc) {
	if f != nil {
		once.Do(func() {
			PanicHandler = f
		})
	}
}

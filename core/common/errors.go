// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package common

import (
	"errors"
	"net"
	"strings"

	"github.com/cocowh/muxcore/core/constant"
)

var (
	ErrPanic                    = errors.New(constant.ErrMessagePanic)
	ErrConnectionClosed         = errors.New(constant.ErrMessageConnectionClosed)
	ErrClientClosed             = errors.New(constant.ErrMessageClientClosed)
	ErrServerClosed             = errors.New(constant.ErrMessageServerClosed)
	ErrConflictWithEventHandler = errors.New(constant.ErrMessageConflictWithEventHandler)
	ErrConfigNotFound           = errors.New(constant.ErrMessageConfigNotFound)
	ErrHeartbeatTimeout         = errors.New(constant.ErrMessageHeartbeatTimeout)
	ErrInvalidTLSConfig         = errors.New("invalid TLS config")
)

func IsTemporaryError(err error) bool {
	if netErr, ok := err.(net.Error); ok {
		if netErr.Timeout() {
			return true
		}
	}

	errStr := err.Error()
	return strings.Contains(errStr, "temporary error") ||
		strings.Contains(errStr, "connection reset by peer") ||
		strings.Contains(errStr, "broken pipe") ||
		strings.Contains(errStr, "too many open files")
}

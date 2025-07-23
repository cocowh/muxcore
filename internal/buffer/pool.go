// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package buffer

import (
	"encoding/binary"
	"sync"
)

var (
	bytesBufferPool = sync.Pool{New: func() interface{} {
		return &BytesBuffer{
			temp:  make([]byte, 8),
			order: binary.BigEndian,
		}
	}}
)

func AcquireBytesBuffer(initSize int) *BytesBuffer {
	bb := bytesBufferPool.Get().(*BytesBuffer)
	if bb.buf == nil {
		bb.buf = make([]byte, initSize)
	}
	return bb
}

func ReleaseBytesBuffer(b *BytesBuffer) {
	if b != nil {
		b.Reset()
		bytesBufferPool.Put(b)
	}
}

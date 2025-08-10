// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package buffer



// AcquireBytesBuffer 从全局统一缓冲区池获取缓冲区
func AcquireBytesBuffer(initSize int) *BytesBuffer {
	buf := GetGlobalUnifiedPool().Get()
	// 如果需要的大小超过当前缓冲区大小，重新分配
	if initSize > buf.Cap() {
		buf.buf = make([]byte, initSize)
	}
	return buf
}

// ReleaseBytesBuffer 归还缓冲区到全局统一缓冲区池
func ReleaseBytesBuffer(b *BytesBuffer) {
	if b != nil {
		GetGlobalUnifiedPool().Put(b)
	}
}

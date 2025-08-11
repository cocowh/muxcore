// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package buffer

import (
	"encoding/binary"
	"errors"
	"io"
)

var (
	ErrNotEnough = errors.New("BytesBuffer: not enough bytes")
	ErrOverflow  = errors.New("BytesBuffer: integer overflow")
)

type BytesBuffer struct {
	buf   []byte
	rpos  int
	wpos  int
	order binary.ByteOrder
	temp  []byte
}

func NewBytesBuffer(size int) *BytesBuffer {
	return NewBytesBufferWithOrder(size, binary.BigEndian)
}

func NewBytesBufferWithOrder(size int, order binary.ByteOrder) *BytesBuffer {
	return &BytesBuffer{
		buf:   make([]byte, size),
		order: order,
		temp:  make([]byte, 8),
	}
}

func CreateBytesBuffer(data []byte) *BytesBuffer {
	return CreateBytesBufferWithOrder(data, binary.BigEndian)
}

func CreateBytesBufferWithOrder(data []byte, order binary.ByteOrder) *BytesBuffer {
	return &BytesBuffer{buf: data,
		order: order,
		temp:  make([]byte, 8),
		wpos:  len(data),
	}
}

func (b *BytesBuffer) SetWPos(pos int) {
	if len(b.buf) < pos {
		b.grow(pos - len(b.buf))
	}
	b.wpos = pos
}

func (b *BytesBuffer) GetWPos() int {
	return b.wpos
}

func (b *BytesBuffer) SetRPos(pos int) {
	b.rpos = pos
}

func (b *BytesBuffer) GetRPos() int {
	return b.rpos
}

func (b *BytesBuffer) WriteByte(c byte) error {
	if len(b.buf) < b.wpos+1 {
		b.grow(1)
	}
	b.buf[b.wpos] = c
	b.wpos++
	return nil
}

func (b *BytesBuffer) WriteString(str string) {
	l := len(str)
	if len(b.buf) < b.wpos+l {
		b.grow(l)
	}
	copy(b.buf[b.wpos:], str)
	b.wpos += l
}

func (b *BytesBuffer) Write(bytes []byte) {
	l := len(bytes)
	if len(b.buf) < b.wpos+l {
		b.grow(l)
	}
	copy(b.buf[b.wpos:], bytes)
	b.wpos += l
}

func (b *BytesBuffer) WriteUint16(u uint16) {
	if len(b.buf) < b.wpos+2 {
		b.grow(2)
	}
	b.order.PutUint16(b.temp, u)
	copy(b.buf[b.wpos:], b.temp[:2])
	b.wpos += 2
}

func (b *BytesBuffer) WriteUint32(u uint32) {
	if len(b.buf) < b.wpos+4 {
		b.grow(4)
	}
	b.order.PutUint32(b.temp, u)
	copy(b.buf[b.wpos:], b.temp[:4])
	b.wpos += 4
}

func (b *BytesBuffer) WriteUint64(u uint64) {
	if len(b.buf) < b.wpos+8 {
		b.grow(8)
	}
	b.order.PutUint64(b.temp, u)
	copy(b.buf[b.wpos:], b.temp[:8])
	b.wpos += 8
}

func (b *BytesBuffer) WriteZigzag32(u uint32) int {
	return b.WriteVarint(uint64((u << 1) ^ uint32(int32(u)>>31)))
}

func (b *BytesBuffer) WriteZigzag64(u uint64) int {
	return b.WriteVarint((u << 1) ^ uint64(int64(u)>>63))
}

func (b *BytesBuffer) WriteVarint(u uint64) int {
	l := 0
	for u >= 1<<7 {
		b.WriteByte(uint8(u&0x7f | 0x80))
		u >>= 7
		l++
	}
	b.WriteByte(uint8(u))
	l++
	return l
}

func (b *BytesBuffer) grow(n int) {
	buf := make([]byte, 2*len(b.buf)+n)
	copy(buf, b.buf[:b.wpos])
	b.buf = buf
}

func (b *BytesBuffer) Bytes() []byte { return b.buf[:b.wpos] }

func (b *BytesBuffer) Read(p []byte) (n int, err error) {
	if b.rpos >= len(b.buf) {
		return 0, io.EOF
	}

	n = copy(p, b.buf[b.rpos:])
	b.rpos += n
	return n, nil
}

func (b *BytesBuffer) ReadFull(p []byte) error {
	if b.Remain() < len(p) {
		return ErrNotEnough
	}
	n := copy(p, b.buf[b.rpos:])
	if n < len(p) {
		return ErrNotEnough
	}
	b.rpos += n
	return nil
}

func (b *BytesBuffer) ReadUint16() (n uint16, err error) {
	if b.Remain() < 2 {
		return 0, ErrNotEnough
	}
	n = b.order.Uint16(b.buf[b.rpos : b.rpos+2])
	b.rpos += 2
	return n, nil
}

func (b *BytesBuffer) ReadInt() (int, error) {
	n, err := b.ReadUint32()
	return int(n), err
}

func (b *BytesBuffer) ReadUint32() (n uint32, err error) {
	if b.Remain() < 4 {
		return 0, ErrNotEnough
	}
	n = b.order.Uint32(b.buf[b.rpos : b.rpos+4])
	b.rpos += 4
	return n, nil
}

func (b *BytesBuffer) ReadUint64() (n uint64, err error) {
	if b.Remain() < 8 {
		return 0, ErrNotEnough
	}
	n = b.order.Uint64(b.buf[b.rpos : b.rpos+8])
	b.rpos += 8
	return n, nil
}

func (b *BytesBuffer) ReadZigzag64() (x uint64, err error) {
	x, err = b.ReadVarint()
	if err != nil {
		return
	}
	x = (x >> 1) ^ uint64(-int64(x&1))
	return
}

func (b *BytesBuffer) ReadZigzag32() (x uint64, err error) {
	x, err = b.ReadVarint()
	if err != nil {
		return
	}
	x = uint64((uint32(x) >> 1) ^ uint32(-int32(x&1)))
	return
}

func (b *BytesBuffer) ReadVarint() (x uint64, err error) {
	var temp byte
	for offset := uint(0); offset < 64; offset += 7 {
		temp, err = b.ReadByte()
		if err != nil {
			return 0, err
		}
		if (temp & 0x80) != 0x80 {
			x |= uint64(temp) << offset
			return x, nil
		}
		x |= uint64(temp&0x7f) << offset
	}
	return 0, ErrOverflow
}

func (b *BytesBuffer) Next(n int) ([]byte, error) {
	m := b.Remain()
	if n > m {
		return nil, ErrNotEnough
	}
	data := b.buf[b.rpos : b.rpos+n]
	b.rpos += n
	return data, nil
}

func (b *BytesBuffer) ReadByte() (byte, error) {
	if b.rpos >= len(b.buf) {
		return 0, io.EOF
	}
	c := b.buf[b.rpos]
	b.rpos++
	return c, nil
}

func (b *BytesBuffer) Reset() {
	b.rpos = 0
	b.wpos = 0
}

func (b *BytesBuffer) Remain() int { return b.wpos - b.rpos }

func (b *BytesBuffer) Len() int { return b.wpos - 0 }

func (b *BytesBuffer) Cap() int { return cap(b.buf) }

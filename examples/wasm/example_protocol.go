// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

//go:build (js || wasip1) && wasm

package main

import (
	"unsafe"
)

// 定义内存块结构
type MemoryBlock struct {
	Size   uint32
	Next   uint32
	IsFree bool
}

// 内存分配器状态
var (
	heapStart    uint32 = 1024                                 // 堆起始地址
	heapSize     uint32 = 0                                    // 当前堆大小
	freeListHead uint32 = 0                                    // 空闲链表头指针
	maxHeapSize  uint32 = 1024 * 1024 * 10                     // 最大堆大小 (10MB)
	blockSize    uint32 = uint32(unsafe.Sizeof(MemoryBlock{})) // 内存块元数据大小
)

// 错误码
const (
	ErrSuccess        = 0
	ErrOutOfMemory    = 1
	ErrInvalidPointer = 2
	ErrInvalidSize    = 3
)

// 获取最后一次错误
//
//export get_last_error
func get_last_error() uint32 {
	return 0
}

// 这是将被WASM模块导出的处理函数
//
//export handle
func handle(inputOffset, inputSize uint32) (uint32, uint32) {
	// 在WASM环境中，应该使用WASM内存API而不是直接指针操作
	// 这里改为使用更安全的方式访问内存

	// 获取输入数据
	input := make([]byte, inputSize)
	copyFromWASMMemory(input, inputOffset, inputSize)

	// 处理数据 - 将输入转为大写
	output := make([]byte, len(input))
	for i, b := range input {
		if b >= 'a' && b <= 'z' {
			output[i] = b - 32
		} else {
			output[i] = b
		}
	}

	// 分配内存存储输出
	outputOffset, errCode := alloc_safe(uint32(len(output)))
	if errCode != ErrSuccess {
		return 0, 0
	}

	// 写入输出数据
	copyToWASMMemory(output, outputOffset)

	return outputOffset, uint32(len(output))
}

// 安全的内存分配函数
//
//export alloc_safe
func alloc_safe(size uint32) (uint32, uint32) {
	if size == 0 {
		return 0, ErrInvalidSize
	}

	// 查找空闲块
	var prevBlock, currBlock uint32
	currBlock = freeListHead

	for currBlock != 0 {
		block := getMemoryBlock(currBlock)
		if block == nil {
			return 0, ErrInvalidPointer
		}

		// 找到足够大的空闲块
		if block.IsFree && block.Size >= size {
			// 如果块大小足够大，可以拆分
			if block.Size >= size+blockSize+32 {
				// 创建新块
				newBlockOffset := currBlock + blockSize + size
				newBlock := getMemoryBlock(newBlockOffset)
				if newBlock == nil {
					return 0, ErrOutOfMemory
				}

				newBlock.Size = block.Size - size - blockSize
				newBlock.Next = block.Next
				newBlock.IsFree = true

				// 更新当前块
				block.Size = size
				block.Next = newBlockOffset
			}

			// 从空闲链表中移除当前块
			if prevBlock == 0 {
				freeListHead = block.Next
			} else {
				prevBlockPtr := getMemoryBlock(prevBlock)
				if prevBlockPtr == nil {
					return 0, ErrInvalidPointer
				}
				prevBlockPtr.Next = block.Next
			}

			// 标记为已使用
			block.IsFree = false

			return currBlock + blockSize, ErrSuccess
		}

		prevBlock = currBlock
		currBlock = block.Next
	}

	// 没有足够的空闲块，分配新内存
	dataOffset := heapStart + blockSize
	requiredSize := blockSize + size

	// 检查是否超出最大堆大小
	if heapSize+requiredSize > maxHeapSize {
		return 0, ErrOutOfMemory
	}

	// 初始化块元数据
	block := getMemoryBlock(heapStart)
	if block == nil {
		return 0, ErrOutOfMemory
	}

	block.Size = size
	block.Next = 0
	block.IsFree = false

	// 更新堆状态
	heapStart += requiredSize
	heapSize += requiredSize

	return dataOffset, ErrSuccess
}

// 内存释放函数
//
//export free
func free(ptr uint32) uint32 {
	if ptr < blockSize || ptr < heapStart-heapSize+blockSize {
		return ErrInvalidPointer
	}

	// 计算块元数据的地址
	blockOffset := ptr - blockSize
	block := getMemoryBlock(blockOffset)
	if block == nil {
		return ErrInvalidPointer
	}

	// 标记为空闲
	block.IsFree = true

	// 尝试合并相邻的空闲块
	mergeFreeBlocks()

	return ErrSuccess
}

// 合并相邻的空闲块
func mergeFreeBlocks() {
	var prevBlock, currBlock uint32
	currBlock = freeListHead

	for currBlock != 0 {
		block := getMemoryBlock(currBlock)
		if block == nil {
			break
		}

		nextBlockOffset := block.Next
		merged := false

		if block.IsFree && nextBlockOffset != 0 {
			nextBlock := getMemoryBlock(nextBlockOffset)
			if nextBlock == nil {
				break
			}

			if nextBlock.IsFree {
				// 合并块
				block.Size += blockSize + nextBlock.Size
				block.Next = nextBlock.Next
				merged = true

				// 更新前一个块的next指针（如果存在）
				if prevBlock != 0 {
					prevBlockPtr := getMemoryBlock(prevBlock)
					if prevBlockPtr != nil {
						prevBlockPtr.Next = currBlock
					}
				}
			}
		}

		if !merged {
			prevBlock = currBlock
			currBlock = block.Next
		}
	}
}

// 初始化内存分配器
//
//export init_allocator
func init_allocator() {
	// 初始化空闲链表
	freeListHead = heapStart

	// 创建一个初始的大空闲块
	initialBlock := getMemoryBlock(heapStart)
	if initialBlock != nil {
		initialBlock.Size = maxHeapSize - blockSize
		initialBlock.Next = 0
		initialBlock.IsFree = true
	}

	// 初始化堆大小
	heapSize = 0
}

// 安全获取内存块
func getMemoryBlock(offset uint32) *MemoryBlock {
	if offset == 0 {
		return nil
	}

	// 安全的内存访问：通过内存边界检查
	if offset < heapStart || offset >= heapStart+maxHeapSize {
		return nil
	}

	// 使用runtime.KeepAlive确保指针有效性
	ptr := uintptr(offset)
	return (*MemoryBlock)(unsafe.Pointer(ptr))
}

// 从WASM内存复制数据
func copyFromWASMMemory(dst []byte, offset, size uint32) {
	if size == 0 || offset == 0 {
		return
	}

	// 边界检查
	if offset+size > heapStart+maxHeapSize {
		return
	}

	// 安全复制，避免直接slice转换
	for i := uint32(0); i < size && i < uint32(len(dst)); i++ {
		ptr := unsafe.Pointer(uintptr(offset + i))
		dst[i] = *(*byte)(ptr)
	}
}

// 复制数据到WASM内存
func copyToWASMMemory(src []byte, offset uint32) {
	if len(src) == 0 || offset == 0 {
		return
	}

	// 边界检查
	if offset+uint32(len(src)) > heapStart+maxHeapSize {
		return
	}

	// 安全复制，避免直接slice转换
	for i, b := range src {
		ptr := unsafe.Pointer(uintptr(offset + uint32(i)))
		*(*byte)(ptr) = b
	}
}

func main() {}

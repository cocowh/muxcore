package governance

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cocowh/muxcore/pkg/errors"
	"github.com/cocowh/muxcore/pkg/logger"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// WASMProtocolLoaderImpl 负责加载和执行WASM协议模块

type WASMProtocolLoaderImpl struct {
	mu         sync.RWMutex
	engine     wazero.Runtime
	modules    map[string]api.Module
	active     bool
	expiration time.Duration
}

// NewWASMProtocolLoader 创建一个新的WASM协议加载器
func NewWASMProtocolLoader() (*WASMProtocolLoaderImpl, error) {
	// 创建WASM运行时
	engine := wazero.NewRuntime(context.Background())

	return &WASMProtocolLoaderImpl{
		engine:     engine,
		modules:    make(map[string]api.Module),
		active:     true,
		expiration: 5 * time.Minute,
	}, nil
}

// LoadProtocol 加载WASM协议模块
func (wpl *WASMProtocolLoaderImpl) LoadProtocol(protocolID, version string, wasmCode []byte) error {
	if !wpl.active {
		return errors.WASMError(errors.ErrCodeWASMRuntimeError, "WASM protocol loader is not active")
	}

	wpl.mu.Lock()
	defer wpl.mu.Unlock()

	// 模块ID由协议ID和版本组成
	moduleID := fmt.Sprintf("%s_%s", protocolID, version)

	// 检查模块是否已加载
	if _, exists := wpl.modules[moduleID]; exists {
		return errors.WASMError(errors.ErrCodeWASMRuntimeError, "protocol module already loaded").WithContext("moduleID", moduleID)
	}

	// 编译WASM模块
	compiledModule, err := wpl.engine.CompileModule(context.Background(), wasmCode)
	if err != nil {
		muxErr := errors.WASMError(errors.ErrCodeWASMCompileError, "failed to compile WASM module").WithCause(err).WithContext("moduleID", moduleID)
		errors.Handle(context.Background(), muxErr)
		return muxErr
	}

	// 实例化模块
	module, err := wpl.engine.InstantiateModule(context.Background(), compiledModule, wazero.NewModuleConfig())
	if err != nil {
		muxErr := errors.WASMError(errors.ErrCodeWASMRuntimeError, "failed to instantiate WASM module").WithCause(err).WithContext("moduleID", moduleID)
		errors.Handle(context.Background(), muxErr)
		return muxErr
	}

	// 存储模块
	wpl.modules[moduleID] = module
	logger.Infof("Loaded WASM protocol module: %s", moduleID)

	// 启动过期检查
	go wpl.checkExpiration(moduleID)

	return nil
}

// GetProtocolInstance 获取协议实例
func (wpl *WASMProtocolLoaderImpl) GetProtocolInstance(protocolID, version string) (api.Module, bool) {
	wpl.mu.RLock()
	defer wpl.mu.RUnlock()

	moduleID := fmt.Sprintf("%s_%s", protocolID, version)
	module, exists := wpl.modules[moduleID]
	return module, exists
}

// InitAllocator 初始化协议模块的内存分配器
func (wpl *WASMProtocolLoaderImpl) InitAllocator(protocolID, version string) error {
	module, exists := wpl.GetProtocolInstance(protocolID, version)
	if !exists {
		return fmt.Errorf("protocol module not found: %s_%s", protocolID, version)
	}

	// 获取初始化函数
	initFunc := module.ExportedFunction("init_allocator")
	if initFunc == nil {
		return errors.WASMError(errors.ErrCodeWASMFunctionNotFound, "'init_allocator' function not found in protocol module")
	}

	// 调用初始化函数
	_, err := initFunc.Call(context.Background())
	if err != nil {
		return fmt.Errorf("failed to call init_allocator function: %w", err)
	}

	logger.Infof("Initialized memory allocator for WASM protocol module: %s_%s", protocolID, version)
	return nil
}

// ExecuteProtocol 执行协议处理
func (wpl *WASMProtocolLoaderImpl) ExecuteProtocol(protocolID, version string, input []byte) ([]byte, error) {
	module, exists := wpl.GetProtocolInstance(protocolID, version)
	if !exists {
		return nil, fmt.Errorf("protocol module not found: %s_%s", protocolID, version)
	}

	// 获取处理函数
	handleFunc := module.ExportedFunction("handle")
	if handleFunc == nil {
		return nil, errors.WASMError(errors.ErrCodeWASMFunctionNotFound, "'handle' function not found in protocol module")
	}

	// 准备输入数据
	inputOffset, inputSize, err := writeInput(module, input)
	if err != nil {
		return nil, fmt.Errorf("failed to write input data: %w", err)
	}

	// 调用处理函数 - 将uint32转换为uint64
	results, err := handleFunc.Call(context.Background(), uint64(inputOffset), uint64(inputSize))
	if err != nil {
		return nil, fmt.Errorf("failed to call handle function: %w", err)
	}

	// 解析结果
	outputOffset := uint32(results[0])
	outputSize := uint32(results[1])
	return readOutput(module, outputOffset, outputSize)
}

// UnloadProtocol 卸载协议模块
func (wpl *WASMProtocolLoaderImpl) UnloadProtocol(protocolID, version string) error {
	wpl.mu.Lock()
	defer wpl.mu.Unlock()

	moduleID := fmt.Sprintf("%s_%s", protocolID, version)
	module, exists := wpl.modules[moduleID]
	if !exists {
		return fmt.Errorf("protocol module not found: %s", moduleID)
	}

	// 关闭模块
	module.Close(context.Background())

	// 从映射中删除
	delete(wpl.modules, moduleID)
	logger.Infof("Unloaded WASM protocol module: %s", moduleID)

	return nil
}

// Activate 激活加载器
func (wpl *WASMProtocolLoaderImpl) Activate() {
	wpl.mu.Lock()
	wpl.active = true
	wpl.mu.Unlock()
}

// Deactivate 停用加载器
func (wpl *WASMProtocolLoaderImpl) Deactivate() {
	wpl.mu.Lock()
	wpl.active = false
	// 关闭所有模块
	for id, module := range wpl.modules {
		module.Close(context.Background())
		delete(wpl.modules, id)
		logger.Infof("Unloaded WASM protocol module during deactivation: %s", id)
	}
	wpl.mu.Unlock()
}

// Close 关闭加载器
func (wpl *WASMProtocolLoaderImpl) Close() error {
	wpl.Deactivate()
	return wpl.engine.Close(context.Background())
}

// 设置过期检查
func (wpl *WASMProtocolLoaderImpl) checkExpiration(moduleID string) {
	timer := time.NewTimer(wpl.expiration)
	<-timer.C

	wpl.mu.RLock()
	_, exists := wpl.modules[moduleID]
	wpl.mu.RUnlock()

	if exists {
		// 检查模块是否被访问过
		// 这里简化处理，实际应实现访问计数
		logger.Debugf("WASM protocol module %s expired, unloading...", moduleID)
		// 分割protocolID和version
		sepIndex := -1
		for i := len(moduleID) - 1; i >= 0; i-- {
			if moduleID[i] == '_' {
				sepIndex = i
				break
			}
		}
		if sepIndex > 0 {
			protocolID := moduleID[:sepIndex]
			version := moduleID[sepIndex+1:]
			// 再次检查模块是否存在，因为可能在定时器触发和获取锁之间已被卸载
			wpl.mu.RLock()
			_, stillExists := wpl.modules[moduleID]
			wpl.mu.RUnlock()
			if stillExists {
				wpl.UnloadProtocol(protocolID, version)
			}
		} else {
			logger.Warnf("Invalid moduleID format: %s, cannot unload", moduleID)
		}
	}
}

// 写入输入数据到WASM内存
func writeInput(module api.Module, input []byte) (uint32, uint32, error) {
	// 获取内存
	memory := module.Memory()
	if memory == nil {
		return 0, 0, errors.WASMError(errors.ErrCodeWASMMemoryError, "module has no memory")
	}

	// 分配内存
	allocSafe := module.ExportedFunction("alloc_safe")
	if allocSafe == nil {
		return 0, 0, errors.WASMError(errors.ErrCodeWASMFunctionNotFound, "'alloc_safe' function not found")
	}

	results, err := allocSafe.Call(context.Background(), uint64(len(input)))
	if err != nil {
		return 0, 0, fmt.Errorf("failed to allocate memory: %w", err)
	}

	offset := uint32(results[0])
	errCode := uint32(results[1])
	if errCode != 0 {
		// 如果分配失败，尝试释放已分配的内存（如果有）
		if offset > 0 {
			free := module.ExportedFunction("free")
			if free != nil {
				free.Call(context.Background(), uint64(offset))
			}
		}
		return 0, 0, fmt.Errorf("memory allocation failed with error code: %d", errCode)
	}

	// 写入数据
	if !memory.Write(offset, input) {
		return 0, 0, errors.WASMError(errors.ErrCodeWASMMemoryError, "failed to write input data")
	}

	return offset, uint32(len(input)), nil
}

// 从WASM内存读取输出数据
func readOutput(module api.Module, offset, size uint32) ([]byte, error) {
	// 获取内存
	memory := module.Memory()
	if memory == nil {
		return nil, errors.WASMError(errors.ErrCodeWASMMemoryError, "module has no memory")
	}

	// 读取数据
	bytesRead, ok := memory.Read(offset, size)
	if !ok {
		return nil, errors.WASMError(errors.ErrCodeWASMMemoryError, "failed to read output data")
	}
	// 确保读取了所有请求的数据
	if len(bytesRead) < int(size) {
		return nil, fmt.Errorf("only read %d bytes out of %d requested", len(bytesRead), size)
	}

	output := bytesRead

	// 释放内存
	free := module.ExportedFunction("free")
	if free != nil {
		_, err := free.Call(context.Background(), uint64(offset))
		if err != nil {
			logger.Warnf("Failed to free memory at offset %d: %v", offset, err)
		}
	} else {
		logger.Warnf("'free' function not found, memory at offset %d may not be released", offset)
	}

	return output, nil
}

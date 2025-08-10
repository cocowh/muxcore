package main

import (
	"fmt"
	"io/ioutil"

	"github.com/cocowh/muxcore/core/governance"
	"github.com/cocowh/muxcore/pkg/logger"
)

func main() {
	// 创建WASM协议加载器
	wasmLoader, err := governance.NewWASMProtocolLoader()
	if err != nil {
		logger.Fatalf("Failed to create WASM protocol loader: %v", err)
	}
	defer wasmLoader.Close()

	// 创建协议管理器（注意：为了简化测试，这里传入nil作为trustStore和policyEngine）
	pm := governance.NewProtocolManager(nil, nil, wasmLoader)
	if pm == nil {
		logger.Fatalf("Failed to create protocol manager")
	}

	// 读取WASM模块文件（注意：这个文件需要先通过Go编译成WASM）
	// 编译命令: GOOS=js GOARCH=wasm go build -o example_protocol.wasm ../wasm/example_protocol.go
	wasmCode, err := ioutil.ReadFile("../wasm/example_protocol.wasm")
	if err != nil {
		logger.Fatalf("Failed to read WASM file: %v\nPlease compile the example protocol first using:\nGOOS=js GOARCH=wasm go build -o example_protocol.wasm ../wasm/example_protocol.go", err)
	}

	// 加载WASM协议
	protocolID := "example"
	version := "1.0.0"
	err = pm.LoadWASMProtocol(protocolID, version, wasmCode)
	if err != nil {
		logger.Fatalf("Failed to load WASM protocol: %v", err)
	}

	// 初始化内存分配器
	err = wasmLoader.InitAllocator(protocolID, version)
	if err != nil {
		logger.Fatalf("Failed to initialize allocator: %v", err)
	}

	// 执行WASM协议
	input := []byte("Hello, WASM Protocol!")
	result, err := pm.ExecuteWASMProtocol(protocolID, version, input)
	if err != nil {
		logger.Fatalf("Failed to execute WASM protocol: %v", err)
	}

	// 输出结果
	fmt.Printf("Input: %s\n", input)
	fmt.Printf("Output: %s\n", result)

	// 测试成功
	fmt.Println("WASM protocol test completed successfully!")
}

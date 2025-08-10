# WASM协议热加载示例

本目录包含了如何使用muxcore中WASM协议热加载功能的示例。

## 什么是WASM协议热加载

WASM协议热加载允许在不重启muxcore服务的情况下，动态加载和更新协议处理逻辑。这通过WebAssembly(WASM)技术实现，使协议处理代码可以被编译为轻量级的二进制模块，并在运行时加载执行。

## 编译示例协议

1. 确保你已经安装了Go 1.21+，因为需要支持WASM编译。

2. 编译示例协议为WASM模块：

```bash
GOOS=wasip1 GOARCH=wasm go build -o example_protocol.wasm example_protocol.go
```

## 加载WASM协议

使用以下代码可以加载和测试WASM协议：

```go
package main

import (
	"context"
	"fmt"
	"io/ioutil"

	"github.com/cocowh/muxcore/core/governance"
	"github.com/cocowh/muxcore/pkg/logger"
)

func main() {
	// 初始化日志
	logger.InitDefaultLogger()

	// 创建WASM协议加载器
	wasmLoader, err := governance.NewWASMProtocolLoader()
	if err != nil {
		logger.Fatalf("Failed to create WASM protocol loader: %v", err)
	}
	defer wasmLoader.Close()

	// 创建协议管理器
	// 注意：这里简化了示例，实际应用中需要提供可信仓库和策略引擎
	pm := governance.NewProtocolManager(nil, nil, wasmLoader)

	// 读取WASM模块
	wasmCode, err := ioutil.ReadFile("example_protocol.wasm")
	if err != nil {
		logger.Fatalf("Failed to read WASM module: %v", err)
	}

	// 加载WASM协议
	protocolID := "example" 
	version := "1.0.0"
	if err := pm.LoadWASMProtocol(protocolID, version, wasmCode); err != nil {
		logger.Fatalf("Failed to load WASM protocol: %v", err)
	}

	// 开始测试协议
	if err := pm.StartProtocolTesting(protocolID, version); err != nil {
		logger.Fatalf("Failed to start protocol testing: %v", err)
	}

	// 设置流量比例
	if err := pm.UpdateProtocolTraffic(protocolID, version, 100); err != nil {
		logger.Fatalf("Failed to update protocol traffic: %v", err)
	}

	// 测试协议执行
	input := []byte("Hello, muxcore!")
	result, err := pm.ExecuteWASMProtocol(protocolID, version, input)
	if err != nil {
		logger.Fatalf("Failed to execute WASM protocol: %v", err)
	}

	fmt.Printf("Input: %s\n", input)
	fmt.Printf("Output: %s\n", result)
}
```

## 创建自定义WASM协议

要创建自己的WASM协议，需要遵循以下步骤：

1. 编写一个Go文件，实现`handle`函数，该函数将接收输入数据并返回处理结果。
2. 确保导出`handle`函数，使用`//export handle`注释。
3. 实现内存管理函数`alloc`和`free`，或者使用WASM运行时提供的内存分配器。
4. 编译为WASM模块，使用`GOOS=wasip1 GOARCH=wasm`环境变量。

### 协议接口规范

WASM协议模块必须实现以下接口：

- `handle(inputOffset, inputSize uint32) (outputOffset, outputSize uint32)`: 处理输入数据并返回输出数据。
- `alloc(size uint32) uint32`: 分配内存。
- `free(ptr uint32)`: 释放内存。

## 性能注意事项

1. WASM模块的加载和实例化有一定开销，建议在系统空闲时进行。
2. WASM执行比原生代码慢，对于性能关键的协议，建议仍使用原生实现。
3. 确保合理管理WASM模块的生命周期，及时卸载不再使用的模块。

## 安全考虑

1. 只加载来自可信来源的WASM模块。
2. 实现资源限制，防止WASM模块过度消耗内存或CPU。
3. 考虑使用沙箱环境运行WASM模块，隔离潜在的恶意代码。
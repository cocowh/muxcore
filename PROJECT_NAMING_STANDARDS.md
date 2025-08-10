# MuxCore 项目命名规范

## 概述

本文档定义了 MuxCore 流量代理项目的命名规范，旨在提高代码可读性、维护性和团队协作效率。

## 1. 项目整体命名

### 1.1 项目名称
- **当前**: `muxcore`
- **建议**: 保持 `muxcore`，符合 Go 项目命名惯例（小写、简洁）

### 1.2 模块路径
- **当前**: `github.com/cocowh/muxcore`
- **建议**: 考虑使用更专业的域名，如 `github.com/muxcore/muxcore` 或企业域名

## 2. 包（Package）命名规范

### 2.1 基本原则
- 使用小写字母
- 简洁明了，避免缩写
- 单数形式
- 避免与标准库冲突

### 2.2 当前包结构分析

#### ✅ 规范的包名
```
core/
├── api/          # API 相关
├── bus/          # 消息总线
├── config/       # 配置管理
├── detector/     # 协议检测
├── listener/     # 监听器
├── router/       # 路由
├── security/     # 安全
└── websocket/    # WebSocket
```

#### ⚠️ 需要改进的包名
```
core/
├── common/       # 建议改为 shared 或 utils
├── handlers/     # 建议改为 handler（单数）
├── pool/         # 建议改为 pools 或保持 pool
└── net/          # 与标准库冲突，建议改为 network
```

## 3. 文件命名规范

### 3.1 基本原则
- 使用小写字母和下划线
- 文件名应描述其主要功能
- 避免过长的文件名

### 3.2 当前文件命名分析

#### ✅ 规范的文件名
```
config.go
listener.go
interceptor.go
protocol_detector.go
connection_pool.go
security_manager.go
```

#### ⚠️ 需要改进的文件名
```
# HTTP 处理器包名不一致
core/http/handler.go          # package handlers
core/http/http_optimizer.go   # package handlers
# 建议统一为 package http
```

## 4. 结构体和接口命名规范

### 4.1 结构体命名
- 使用 PascalCase（大驼峰）
- 名称应清晰描述其用途
- 避免不必要的前缀

#### 示例
```go
// ✅ 好的命名
type ConnectionPool struct {}
type SecurityManager struct {}
type ProtocolDetector struct {}

// ❌ 避免的命名
type MuxCoreConnectionPool struct {}  // 不必要的前缀
type CP struct {}                     // 过于简短
```

### 4.2 接口命名
- 使用 PascalCase
- 单一方法接口通常以 -er 结尾
- 多方法接口使用描述性名称

#### 示例
```go
// ✅ 好的命名
type Handler interface {}
type Detector interface {}
type Manager interface {}
type ConnectionPooler interface {}
```

## 5. 方法和函数命名规范

### 5.1 基本原则
- 使用 camelCase（小驼峰）
- 公开方法使用 PascalCase
- 方法名应描述其行为

#### 示例
```go
// ✅ 好的命名
func (p *ConnectionPool) AddConnection() {}
func (p *ConnectionPool) GetActiveConnections() {}
func (d *ProtocolDetector) DetectProtocol() {}

// ❌ 避免的命名
func (p *ConnectionPool) Add() {}        // 过于简短
func (p *ConnectionPool) GetAC() {}      // 缩写不清晰
```

## 6. 变量和常量命名规范

### 6.1 变量命名
- 使用 camelCase
- 局部变量可以简短
- 全局变量应该描述性强

### 6.2 常量命名
- 使用 PascalCase 或 UPPER_CASE
- 相关常量应该分组

#### 示例
```go
// ✅ 常量命名
const (
    DefaultMaxConnections = 1000
    DefaultTimeout        = 30 * time.Second
)

// 或者
const (
    MAX_CONNECTIONS = 1000
    DEFAULT_TIMEOUT = 30
)
```

## 7. 目录结构命名规范

### 7.1 当前目录结构
```
muxcore/
├── cmd/           # 命令行工具
├── core/          # 核心功能模块
├── examples/      # 示例代码
├── internal/      # 内部包
└── pkg/           # 可复用包
```

### 7.2 建议的改进
```
muxcore/
├── cmd/           # 保持不变
├── internal/      # 内部实现
│   ├── core/      # 核心业务逻辑
│   ├── handler/   # 协议处理器
│   ├── network/   # 网络相关（原 net）
│   └── shared/    # 共享工具（原 common）
├── pkg/           # 公开 API
├── examples/      # 示例代码
└── docs/          # 文档
```

## 8. 具体改进建议

### 8.1 立即需要修复的问题

1. **包名不一致问题**
   ```
   # 当前
   core/http/handler.go     -> package handlers
   core/http/http_optimizer.go -> package handlers
   
   # 建议修改为
   core/http/handler.go     -> package http
   core/http/http_optimizer.go -> package http
   ```

2. **与标准库冲突**
   ```
   # 当前
   core/net/ -> package net
   
   # 建议修改为
   core/network/ -> package network
   ```

### 8.2 长期改进建议

1. **重构目录结构**
   - 将核心业务逻辑移到 `internal/` 下
   - 在 `pkg/` 下提供清晰的公开 API
   - 统一优化器文件的命名模式

2. **统一命名模式**
   - 所有优化器文件使用统一后缀：`_optimizer.go`
   - 所有管理器文件使用统一后缀：`_manager.go`
   - 所有处理器文件使用统一后缀：`_handler.go`

## 9. 命名检查清单

在添加新代码时，请检查以下项目：

- [ ] 包名是否小写且简洁
- [ ] 文件名是否使用下划线分隔
- [ ] 结构体名是否使用 PascalCase
- [ ] 方法名是否清晰描述其功能
- [ ] 是否避免了与标准库的命名冲突
- [ ] 是否遵循了项目的命名约定
- [ ] 是否添加了适当的注释

## 10. 工具和自动化

建议使用以下工具来维护命名规范：

1. **golint**: 检查 Go 代码风格
2. **gofmt**: 自动格式化代码
3. **golangci-lint**: 综合代码检查工具
4. **pre-commit hooks**: 提交前自动检查

## 结论

良好的命名规范是项目可维护性的基础。通过遵循这些规范，我们可以：

- 提高代码可读性
- 减少理解成本
- 便于团队协作
- 降低维护难度
- 提升项目专业度

建议分阶段实施这些改进，优先修复明显的问题，然后逐步完善整体结构。
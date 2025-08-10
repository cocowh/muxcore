# Core 模块错误处理统一迁移方案

## 当前错误处理模式分析

### 1. 现有错误处理模式

通过分析 core 目录下的各个模块，发现以下几种错误处理模式：

#### A. 简单日志记录模式
```go
// 示例：detector/protocol_detector.go
logger.Error("No handler found for protocol: ", protocol)

// 示例：listener/listener.go
logger.Error("Failed to accept connection: ", err)
```

#### B. 错误包装返回模式
```go
// 示例：config/config.go
if err := v.ReadInConfig(); err != nil {
    return nil, fmt.Errorf("failed to read config file: %w", err)
}

// 示例：governance/wasm_protocol_loader.go
if err != nil {
    return fmt.Errorf("failed to compile WASM module: %w", err)
}
```

#### C. 直接错误创建模式
```go
// 示例：net/buffered_conn.go
if bc.closed {
    return 0, errors.New("connection closed")
}

// 示例：governance/protocol_manager.go
if !pm.enabled {
    return fmt.Errorf("protocol manager is disabled")
}
```

#### D. 错误忽略模式
```go
// 示例：observability/metrics.go
err := http.ListenAndServe(metricsAddr, nil)
// 错误被忽略，没有处理
```

### 2. 问题分析

1. **不一致性**: 不同模块使用不同的错误处理方式
2. **缺乏分类**: 错误没有按照类型和严重程度分类
3. **缺乏上下文**: 错误信息缺乏足够的上下文信息
4. **难以监控**: 无法统一收集和分析错误
5. **恢复能力差**: 缺乏自动错误恢复机制

## 迁移策略

### 阶段一：核心模块迁移

优先迁移以下核心模块：
1. `config` - 配置管理
2. `listener` - 连接监听
3. `net/buffered_conn` - 网络连接
4. `detector/protocol_detector` - 协议检测

### 阶段二：业务模块迁移

1. `governance` - 治理模块
2. `handlers` - 处理器管理
3. `http/grpc/websocket` - 协议处理器

### 阶段三：辅助模块迁移

1. `observability` - 可观测性
2. `security` - 安全模块
3. `performance` - 性能模块
4. `reliability` - 可靠性模块

## 具体迁移方案

### 1. 错误分类映射

| 模块 | 原错误类型 | 新错误分类 | 错误码范围 |
|------|------------|------------|------------|
| config | 配置读取/验证错误 | CategoryConfig | 5000-5999 |
| listener | 网络监听错误 | CategoryNetwork | 2000-2999 |
| net | 连接/IO错误 | CategoryNetwork | 2000-2999 |
| detector | 协议检测错误 | CategoryProtocol | 3000-3999 |
| governance | WASM/策略错误 | CategoryGovernance | 10000-10999 |
| handlers | 处理器错误 | CategorySystem | 1000-1999 |
| security | 安全验证错误 | CategoryAuth | 6000-6999 |
| performance | 资源管理错误 | CategorySystem | 1000-1999 |

### 2. 迁移步骤

#### 步骤1：引入错误管理包
```go
import (
    "github.com/cocowh/muxcore/pkg/errors"
)
```

#### 步骤2：替换错误创建
```go
// 原代码
return fmt.Errorf("failed to read config file: %w", err)

// 新代码
return errors.ConfigError(errors.ErrCodeConfigParseError, "failed to read config file").WithCause(err)
```

#### 步骤3：添加错误处理
```go
// 原代码
logger.Error("Failed to accept connection: ", err)

// 新代码
muxErr := errors.Convert(err)
errors.Handle(ctx, muxErr.WithContext("operation", "accept_connection"))
```

#### 步骤4：集成中间件
```go
// 在适当的地方初始化错误管理
func init() {
    errors.InitBuiltinHandlers(shutdownChannel)
}
```

### 3. 具体模块迁移计划

#### config 模块
- 配置文件读取错误 → `ErrCodeConfigNotFound`
- 配置解析错误 → `ErrCodeConfigParseError`
- 配置验证错误 → `ErrCodeConfigValidationError`

#### listener 模块
- 监听启动错误 → `ErrCodeNetworkConnectionRefused`
- 连接接受错误 → `ErrCodeNetworkTimeout`

#### net 模块
- 连接关闭错误 → `ErrCodeNetworkConnectionLost`
- 读写超时错误 → `ErrCodeNetworkTimeout`

#### detector 模块
- 协议检测失败 → `ErrCodeProtocolInvalid`
- 处理器未找到 → `ErrCodeProtocolUnsupported`

#### governance 模块
- WASM编译错误 → `ErrCodeWASMCompileError`
- WASM运行错误 → `ErrCodeWASMRuntimeError`
- 策略违反错误 → `ErrCodeGovernancePolicyViolation`

## 迁移时间表

| 阶段 | 模块 | 预计时间 | 负责人 |
|------|------|----------|--------|
| 1 | config, listener, net | 1-2天 | 开发团队 |
| 2 | detector, governance | 2-3天 | 开发团队 |
| 3 | handlers, protocols | 2-3天 | 开发团队 |
| 4 | 其他模块 | 1-2天 | 开发团队 |
| 5 | 测试和优化 | 1-2天 | 开发团队 |

## 验证方案

### 1. 单元测试
- 为每个迁移的模块编写错误处理测试
- 验证错误分类和错误码的正确性

### 2. 集成测试
- 测试错误在模块间的传播
- 验证错误恢复机制

### 3. 性能测试
- 确保错误处理不影响系统性能
- 验证错误处理的内存使用

### 4. 监控验证
- 验证错误指标收集
- 确认错误报告功能

## 风险控制

### 1. 向后兼容
- 保持原有错误接口的兼容性
- 渐进式迁移，避免破坏性变更

### 2. 回滚方案
- 保留原有错误处理代码的备份
- 提供快速回滚机制

### 3. 监控告警
- 监控迁移过程中的错误率变化
- 设置异常告警机制

## 预期收益

1. **统一性**: 所有模块使用统一的错误处理方式
2. **可观测性**: 完整的错误监控和分析能力
3. **可靠性**: 自动错误恢复和降级机制
4. **可维护性**: 清晰的错误分类和上下文信息
5. **扩展性**: 支持自定义错误处理器和报告器
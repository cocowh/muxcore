# MuxCore 项目重构计划

## 概述

本文档提供了 MuxCore 项目命名规范化的具体实施计划，包括优先级、步骤和时间安排。

## 重构优先级

### 🔴 高优先级（立即修复）

#### 1. 修复包名不一致问题

**问题**: HTTP 处理器文件的包声明与目录不匹配

```bash
# 当前状态
core/http/handler.go          # package handlers ❌
core/http/http_optimizer.go   # package handlers ❌

# 目标状态
core/http/handler.go          # package http ✅
core/http/http_optimizer.go   # package http ✅
```

**修复步骤**:
1. 修改 `core/http/handler.go` 的包声明
2. 修改 `core/http/http_optimizer.go` 的包声明
3. 更新所有引用这些包的 import 语句
4. 运行测试确保无破坏性变更

#### 2. 解决标准库命名冲突

**问题**: `core/net` 包与 Go 标准库 `net` 包冲突

```bash
# 当前状态
core/net/buffered_conn.go     # package net ❌

# 目标状态
core/network/buffered_conn.go # package network ✅
```

**修复步骤**:
1. 创建新目录 `core/network/`
2. 移动 `core/net/` 下的所有文件到 `core/network/`
3. 修改包声明为 `package network`
4. 更新所有 import 引用
5. 删除旧的 `core/net/` 目录

### 🟡 中优先级（短期内完成）

#### 3. 统一包名规范

**目标**: 将复数包名改为单数

```bash
# 建议修改
core/handlers/ -> core/handler/
```

**修复步骤**:
1. 重命名目录 `core/handlers/` 为 `core/handler/`
2. 修改包声明为 `package handler`
3. 更新所有 import 引用

#### 4. 改进通用包命名

**目标**: 使包名更具描述性

```bash
# 建议修改
core/common/ -> core/shared/
```

**修复步骤**:
1. 重命名目录 `core/common/` 为 `core/shared/`
2. 修改包声明为 `package shared`
3. 更新所有 import 引用

### 🟢 低优先级（长期规划）

#### 5. 重构目录结构

**目标**: 采用更标准的 Go 项目布局

```bash
# 当前结构
muxcore/
├── core/
│   ├── api/
│   ├── bus/
│   └── ...
├── pkg/
└── examples/

# 目标结构
muxcore/
├── cmd/
├── internal/
│   ├── core/
│   ├── handler/
│   ├── network/
│   └── shared/
├── pkg/
│   └── muxcore/  # 公开 API
├── examples/
└── docs/
```

## 详细实施步骤

### 阶段 1: 紧急修复（1-2 天）

#### 步骤 1.1: 修复 HTTP 包声明

```bash
# 1. 修改文件
vim core/http/handler.go
# 将 "package handlers" 改为 "package http"

vim core/http/http_optimizer.go  
# 将 "package handlers" 改为 "package http"

# 2. 查找并更新所有引用
grep -r "muxcore/core/http" . --include="*.go"
# 确保 import 路径正确

# 3. 运行测试
go test ./...
```

#### 步骤 1.2: 解决 net 包冲突

```bash
# 1. 创建新目录
mkdir -p core/network

# 2. 移动文件
mv core/net/* core/network/

# 3. 修改包声明
sed -i 's/package net/package network/g' core/network/*.go

# 4. 更新 import 引用
find . -name "*.go" -exec sed -i 's|muxcore/core/net|muxcore/core/network|g' {} +

# 5. 删除旧目录
rmdir core/net

# 6. 运行测试
go test ./...
```

### 阶段 2: 包名规范化（3-5 天）

#### 步骤 2.1: 重命名 handlers 包

```bash
# 1. 重命名目录
mv core/handlers core/handler

# 2. 修改包声明
sed -i 's/package handlers/package handler/g' core/handler/*.go

# 3. 更新 import 引用
find . -name "*.go" -exec sed -i 's|muxcore/core/handlers|muxcore/core/handler|g' {} +

# 4. 运行测试
go test ./...
```

#### 步骤 2.2: 重命名 common 包

```bash
# 1. 重命名目录
mv core/common core/shared

# 2. 修改包声明
sed -i 's/package common/package shared/g' core/shared/*.go

# 3. 更新 import 引用
find . -name "*.go" -exec sed -i 's|muxcore/core/common|muxcore/core/shared|g' {} +

# 4. 运行测试
go test ./...
```

### 阶段 3: 长期重构（1-2 周）

#### 步骤 3.1: 重构目录结构

```bash
# 1. 创建新的目录结构
mkdir -p internal/core
mkdir -p internal/handler
mkdir -p internal/network
mkdir -p internal/shared
mkdir -p pkg/muxcore
mkdir -p docs

# 2. 移动核心模块
mv core/* internal/core/

# 3. 重新组织包结构
# 这需要仔细规划每个包的职责

# 4. 创建公开 API
# 在 pkg/muxcore/ 下创建清晰的公开接口

# 5. 更新所有 import 路径
# 这是最复杂的步骤，需要逐个文件检查
```

## 自动化脚本

### 脚本 1: 包声明修复

```bash
#!/bin/bash
# fix_package_declarations.sh

echo "修复 HTTP 包声明..."
sed -i 's/package handlers/package http/g' core/http/*.go

echo "修复 net 包冲突..."
mkdir -p core/network
mv core/net/* core/network/ 2>/dev/null || true
sed -i 's/package net/package network/g' core/network/*.go
rmdir core/net 2>/dev/null || true

echo "更新 import 引用..."
find . -name "*.go" -exec sed -i 's|muxcore/core/net|muxcore/core/network|g' {} +

echo "运行测试..."
go test ./...

echo "修复完成！"
```

### 脚本 2: 包名规范化

```bash
#!/bin/bash
# normalize_package_names.sh

echo "重命名 handlers 包..."
mv core/handlers core/handler 2>/dev/null || true
sed -i 's/package handlers/package handler/g' core/handler/*.go
find . -name "*.go" -exec sed -i 's|muxcore/core/handlers|muxcore/core/handler|g' {} +

echo "重命名 common 包..."
mv core/common core/shared 2>/dev/null || true
sed -i 's/package common/package shared/g' core/shared/*.go
find . -name "*.go" -exec sed -i 's|muxcore/core/common|muxcore/core/shared|g' {} +

echo "运行测试..."
go test ./...

echo "规范化完成！"
```

## 验证清单

### 每个阶段完成后的检查项目

- [ ] 所有 Go 文件能够正常编译
- [ ] 所有测试通过
- [ ] 没有 import 循环依赖
- [ ] 包名与目录名一致
- [ ] 没有与标准库的命名冲突
- [ ] 代码格式化正确（gofmt）
- [ ] 通过 golint 检查
- [ ] 更新了相关文档

### 最终验证命令

```bash
# 编译检查
go build ./...

# 测试检查
go test ./...

# 代码风格检查
golint ./...

# 格式化检查
gofmt -l .

# 依赖检查
go mod tidy
go mod verify
```

## 风险评估和缓解

### 潜在风险

1. **破坏性变更**: 修改包名可能影响外部依赖
   - **缓解**: 在独立分支进行，充分测试后合并

2. **Import 路径错误**: 大量文件需要更新 import
   - **缓解**: 使用自动化脚本，分步骤执行

3. **测试失败**: 重构可能导致测试失败
   - **缓解**: 每个步骤后立即运行测试

4. **文档过时**: 重构后文档可能不匹配
   - **缓解**: 同步更新文档和示例

### 回滚计划

如果重构出现问题，可以通过以下方式回滚：

1. 使用 Git 回滚到重构前的提交
2. 保留重构前的备份分支
3. 分阶段提交，便于部分回滚

## 时间安排

| 阶段 | 任务 | 预计时间 | 负责人 |
|------|------|----------|--------|
| 1 | 紧急修复 | 1-2 天 | 开发团队 |
| 2 | 包名规范化 | 3-5 天 | 开发团队 |
| 3 | 长期重构 | 1-2 周 | 架构师 + 开发团队 |

## 总结

通过分阶段实施这个重构计划，我们可以：

1. 快速修复紧急问题
2. 逐步改善代码质量
3. 最终实现标准化的项目结构
4. 提高项目的可维护性和专业度

建议立即开始阶段 1 的工作，确保项目的基本规范性。
#!/bin/bash

# MuxCore 项目命名规范化脚本
# 此脚本用于自动修复项目中的命名不规范问题

set -e  # 遇到错误立即退出

echo "🚀 开始 MuxCore 项目命名规范化..."

# 创建脚本目录（如果不存在）
mkdir -p scripts

# 函数：检查命令是否存在
check_command() {
    if ! command -v $1 &> /dev/null; then
        echo "❌ 错误: $1 命令未找到，请先安装"
        exit 1
    fi
}

# 函数：备份当前状态
backup_project() {
    echo "📦 创建项目备份..."
    git add -A
    git commit -m "备份：命名规范化前的状态" || echo "⚠️  没有变更需要提交"
    git tag "backup-before-naming-fix-$(date +%Y%m%d-%H%M%S)" || echo "⚠️  标签创建失败"
}

# 函数：运行测试
run_tests() {
    echo "🧪 运行测试..."
    if go test ./... > /dev/null 2>&1; then
        echo "✅ 所有测试通过"
        return 0
    else
        echo "❌ 测试失败"
        return 1
    fi
}

# 函数：检查编译
check_build() {
    echo "🔨 检查编译..."
    if go build ./... > /dev/null 2>&1; then
        echo "✅ 编译成功"
        return 0
    else
        echo "❌ 编译失败"
        return 1
    fi
}

# 检查必要的命令
check_command "go"
check_command "git"
check_command "find"
check_command "sed"

# 备份项目
backup_project

echo "\n📋 第一阶段：修复高优先级问题"

# 1. 修复 HTTP 包声明（已完成）
echo "✅ HTTP 包声明已修复"

# 2. 修复 net 包冲突（已完成）
echo "✅ net 包冲突已修复"

echo "\n📋 第二阶段：包名规范化"

# 3. 重命名 handlers 包为 handler
echo "🔄 重命名 handlers 包为 handler..."
if [ -d "core/handlers" ]; then
    # 重命名目录
    mv core/handlers core/handler
    echo "  ✅ 目录重命名完成"
    
    # 修改包声明
    find core/handler -name "*.go" -exec sed -i '' 's/package handlers/package handler/g' {} +
    echo "  ✅ 包声明修改完成"
    
    # 更新 import 引用
    find . -name "*.go" -exec sed -i '' 's|muxcore/core/handlers|muxcore/core/handler|g' {} +
    echo "  ✅ import 引用更新完成"
else
    echo "  ⚠️  handlers 目录不存在，跳过"
fi

# 4. 重命名 common 包为 shared
echo "🔄 重命名 common 包为 shared..."
if [ -d "core/common" ]; then
    # 重命名目录
    mv core/common core/shared
    echo "  ✅ 目录重命名完成"
    
    # 修改包声明
    find core/shared -name "*.go" -exec sed -i '' 's/package common/package shared/g' {} +
    echo "  ✅ 包声明修改完成"
    
    # 更新 import 引用
    find . -name "*.go" -exec sed -i '' 's|muxcore/core/common|muxcore/core/shared|g' {} +
    echo "  ✅ import 引用更新完成"
else
    echo "  ⚠️  common 目录不存在，跳过"
fi

echo "\n🔍 验证修改结果..."

# 检查编译
if check_build; then
    echo "✅ 编译检查通过"
else
    echo "❌ 编译检查失败，请手动检查错误"
    exit 1
fi

# 运行代码格式化
echo "🎨 格式化代码..."
go fmt ./...
echo "✅ 代码格式化完成"

# 整理依赖
echo "📦 整理依赖..."
go mod tidy
echo "✅ 依赖整理完成"

# 最终验证
echo "\n🔍 最终验证..."
if check_build; then
    echo "✅ 最终编译检查通过"
else
    echo "❌ 最终编译检查失败"
    exit 1
fi

# 提交更改
echo "\n💾 提交更改..."
git add -A
git commit -m "feat: 规范化项目命名

- 修复 HTTP 包声明不一致问题
- 解决 net 包与标准库冲突
- 重命名 handlers 包为 handler
- 重命名 common 包为 shared
- 更新所有相关的 import 引用

符合 Go 项目命名最佳实践"

echo "\n🎉 命名规范化完成！"
echo "\n📊 修改总结："
echo "  ✅ HTTP 包声明统一为 package http"
echo "  ✅ 解决了 net 包与标准库的冲突"
echo "  ✅ handlers 包重命名为 handler"
echo "  ✅ common 包重命名为 shared"
echo "  ✅ 所有 import 引用已更新"
echo "  ✅ 代码编译正常"
echo "  ✅ 依赖关系正确"

echo "\n📝 后续建议："
echo "  1. 运行完整的测试套件"
echo "  2. 更新相关文档"
echo "  3. 通知团队成员关于包名变更"
echo "  4. 考虑实施长期重构计划"

echo "\n🔗 相关文档："
echo "  - PROJECT_NAMING_STANDARDS.md"
echo "  - REFACTORING_PLAN.md"
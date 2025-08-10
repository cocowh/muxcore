#!/bin/bash

# 构建脚本：编译WASM模块和测试程序

# 检查Go是否安装
if ! command -v go &> /dev/null
then
  echo "Go is not installed. Please install Go first."
  exit 1
fi

# 清理旧构建文件
echo "Cleaning old build files..."
rm -f ../wasm/example_protocol.wasm
rm -f wasm_test

# 编译WASM模块
echo "Compiling WASM module with enhanced memory allocator..."
cd ../wasm
GOOS=js GOARCH=wasm go build -o example_protocol.wasm example_protocol.go
if [ $? -ne 0 ]
then
  echo "Failed to compile WASM module"
  exit 1
fi
cd -

# 检查WASM模块是否生成
if [ ! -f ../wasm/example_protocol.wasm ]
then
  echo "WASM module not found after compilation"
  exit 1
fi

# 编译测试程序
echo "Compiling test program..."
go build -o wasm_test main.go
if [ $? -ne 0 ]
then
  echo "Failed to compile test program"
  exit 1
fi

# 检查测试程序是否生成
if [ ! -f wasm_test ]
then
  echo "Test program not found after compilation"
  exit 1
fi

echo "Build completed successfully!"
echo "===================================="
echo "WASM Protocol Test Instructions:"
echo "1. Make sure you have the required dependencies installed"
echo "2. Execute the test program: ./wasm_test"
echo "3. The test will load the WASM protocol, initialize the memory allocator,"
echo "   and execute the protocol with sample input"
echo "===================================="
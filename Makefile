# Makefile for muxcore

# 构建应用程序
build:
	go build -o bin/muxcore ./cmd/muxcore

# 运行应用程序
run:
	go run ./cmd/muxcore

# 清理构建产物
clean:
	rm -rf bin/

# 构建并运行
all: build run

.PHONY: build run clean all
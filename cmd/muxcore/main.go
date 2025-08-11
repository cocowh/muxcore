// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/cocowh/muxcore/core/control"
	"github.com/cocowh/muxcore/pkg/logger"
	"github.com/spf13/cobra"
)

var (
	configPath string
	verbose    bool
	logLevel   string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "muxcore",
	Short: "MuxCore is a high-performance multiplexing proxy server",
	Long: `MuxCore is a high-performance, protocol-agnostic multiplexing proxy server
that supports dynamic protocol detection, WASM-based protocol extensions,
and advanced traffic management capabilities.`,
	Version: "1.0.0",
}

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the MuxCore server",
	Long:  `Start the MuxCore server with the specified configuration.`,
	RunE:  runServer,
}

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of MuxCore",
	Long:  `Print the version number and build information of MuxCore.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("MuxCore version %s\n", rootCmd.Version)
	},
}

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configuration management commands",
	Long:  `Commands for managing MuxCore configuration files.`,
}

// validateConfigCmd represents the validate config command
var validateConfigCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configuration file",
	Long:  `Validate the syntax and semantics of a MuxCore configuration file.`,
	RunE:  validateConfig,
}

func init() {
	// Add subcommands
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(validateConfigCmd)

	// Global flags
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "config.yaml", "path to configuration file")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose logging")
	rootCmd.PersistentFlags().StringVarP(&logLevel, "log-level", "l", "info", "set log level (trace, debug, info, warn, error, fatal)")

	// Serve command flags
	serveCmd.Flags().StringP("address", "a", ":8080", "server listen address")
	serveCmd.Flags().StringP("cert", "", "", "TLS certificate file")
	serveCmd.Flags().StringP("key", "", "", "TLS private key file")

	// Validate command flags
	validateConfigCmd.Flags().StringVarP(&configPath, "file", "f", "config.yaml", "configuration file to validate")
}

func runServer(cmd *cobra.Command, args []string) error {
	// 创建控制平面构建器并初始化日志系统
	builder, err := control.NewControlPlaneBuilderWithLogConfig(configPath, logLevel, verbose)
	if err != nil {
		return fmt.Errorf("failed to create control plane builder: %w", err)
	}

	logger.Info("Starting MuxCore server...")
	logger.Infof("Using configuration file: %s", configPath)

	// 构建控制平面
	controlPlane, err := builder.Build()
	if err != nil {
		return fmt.Errorf("failed to build control plane: %w", err)
	}

	logger.Info("Control plane created successfully")

	// 启动控制平面
	if err := controlPlane.Start(); err != nil {
		return fmt.Errorf("failed to start control plane: %w", err)
	}

	logger.Info("MuxCore server started successfully")

	// 等待中断信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	logger.Info("Shutting down MuxCore server...")

	// 停止控制平面
	if err := controlPlane.Stop(); err != nil {
		logger.Errorf("Error during shutdown: %v", err)
		return err
	}

	logger.Info("MuxCore server stopped gracefully")
	return nil
}

func validateConfig(cmd *cobra.Command, args []string) error {
	// 基本日志配置用于验证过程
	loggerConfig := &logger.Config{
		Level:  logger.InfoLevel,
		Format: "text",
	}

	if err := logger.InitDefaultLogger(loggerConfig); err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}

	logger.Infof("Validating configuration file: %s", configPath)
	// TODO: 实现配置验证逻辑
	logger.Info("Configuration validation completed successfully")
	return nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

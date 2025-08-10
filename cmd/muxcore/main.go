package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/cocowh/muxcore/core/control"
	"github.com/cocowh/muxcore/pkg/logger"
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
		fmt.Printf("MuxCore v%s\n", rootCmd.Version)
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
	// Global flags
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "config.yaml", "Path to configuration file")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "Log level (trace, debug, info, warn, error, fatal)")

	// Add subcommands
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(validateConfigCmd)

	// Serve command flags
	serveCmd.Flags().String("bind", "", "Override bind address from config")
	serveCmd.Flags().Int("port", 0, "Override port from config")
	serveCmd.Flags().Bool("daemon", false, "Run as daemon")
}

func runServer(cmd *cobra.Command, args []string) error {
	// 初始化日志系统
	var level logger.Level
	switch logLevel {
	case "trace":
		level = logger.TraceLevel
	case "debug":
		level = logger.DebugLevel
	case "info":
		level = logger.InfoLevel
	case "warn":
		level = logger.WarnLevel
	case "error":
		level = logger.ErrorLevel
	case "fatal":
		level = logger.FatalLevel
	default:
		level = logger.InfoLevel
	}

	if verbose {
		level = logger.DebugLevel
	}

	loggerConfig := &logger.Config{
		Level: level,
	}

	if err := logger.InitDefaultLogger(loggerConfig); err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}

	// 创建控制平面
	controlPlane, err := control.New(configPath)
	if err != nil {
		return fmt.Errorf("failed to create control plane: %w", err)
	}

	// 启动控制平面
	if err := controlPlane.Start(); err != nil {
		return fmt.Errorf("failed to start control plane: %w", err)
	}

	// 等待退出信号
	logger.Info("MuxCore server started. Press Ctrl+C to stop.")
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// 停止控制平面
	controlPlane.Stop()
	logger.Info("MuxCore server stopped")
	return nil
}

func validateConfig(cmd *cobra.Command, args []string) error {
	// 初始化日志系统
	if err := logger.InitDefaultLogger(nil); err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}

	// 尝试创建控制平面来验证配置
	_, err := control.New(configPath)
	if err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	fmt.Printf("Configuration file '%s' is valid\n", configPath)
	return nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

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

func init() {
	// Add subcommands
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(versionCmd)

	// Global flags
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "config.yaml", "path to configuration file")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose logging")
	rootCmd.PersistentFlags().StringVarP(&logLevel, "log-level", "l", "info", "set log level (trace, debug, info, warn, error, fatal)")

	// Serve command flags
	serveCmd.Flags().StringP("address", "a", ":8080", "server listen address")
	serveCmd.Flags().StringP("cert", "", "", "TLS certificate file")
	serveCmd.Flags().StringP("key", "", "", "TLS private key file")
}

func runServer(cmd *cobra.Command, args []string) error {
	// create control plane builder with log config
	builder, err := control.NewControlPlaneBuilderWithLogConfig(configPath, logLevel, verbose)
	if err != nil {
		return fmt.Errorf("failed to create control plane builder: %w", err)
	}

	logger.Info("Starting MuxCore server...")
	logger.Infof("Using configuration file: %s", configPath)

	// build control plane
	controlPlane, err := builder.Build()
	if err != nil {
		return fmt.Errorf("failed to build control plane: %w", err)
	}

	logger.Info("Control plane created successfully")

	// start control plane
	if err := controlPlane.Start(); err != nil {
		return fmt.Errorf("failed to start control plane: %w", err)
	}

	logger.Info("MuxCore server started successfully")

	// wait for signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	logger.Info("Shutting down MuxCore server...")

	// stop control plane
	if err := controlPlane.Stop(); err != nil {
		logger.Errorf("Error during shutdown: %v", err)
		return err
	}

	logger.Info("MuxCore server stopped gracefully")
	return nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

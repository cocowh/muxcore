# MuxCore Logger Package

This package provides a flexible and configurable logging system for Go applications.

## Features
- Multiple log levels (Trace, Debug, Info, Warn, Error, Fatal)
- Multiple logger backends (Standard library, Zap)
- Asynchronous logging support
- Log rotation based on time and size
- Configurable log format (JSON, console)
- Support for multiple log files (separate warn/error logs)
- Log compression and cleanup of old logs
- Viper integration for configuration

## Package Structure

```
logger/
├── level.go       # Defines log levels
├── options.go     # Defines configuration struct
├── logger.go      # Core logger implementation
├── std_logger.go  # Standard library logger implementation
├── zap_logger.go  # Zap logger implementation
├── rolling_writer.go # Log rotation implementation
└── example_test.go # Example usage and tests
```

## Usage

### Basic Initialization

```go
import (
    "github.com/yourusername/muxcore/pkg/logger"
)

func init() {
    // Initialize with default configuration
    err := logger.InitLogger(nil)
    if err != nil {
        panic(err)
    }
}

func main() {
    logger.Infof("Application started")
    logger.Debugf("Debug information: %v", someVariable)
    logger.Warnf("Warning: %v", warningMessage)
    logger.Errorf("Error: %v", err)
    // For fatal errors that should exit the application
    // logger.Fatalf("Fatal error: %v", err)
}
```

### Custom Configuration

```go
import (
    "github.com/yourusername/muxcore/pkg/logger"
)

func init() {
    config := &logger.Config{
        LogDir:          "/var/log/myapp",
        BaseName:        "myapp",
        Format:          "json", // or "console"
        Level:           logger.InfoLevel,
        Compress:        true,
        MaxSizeMB:       100,
        MaxBackups:      7,
        MaxAgeDays:      30,
        EnableWarnFile:  true,
        EnableErrorFile: true,
    }

    err := logger.InitLogger(config)
    if err != nil {
        panic(err)
    }
}
```

### Asynchronous Logging

Asynchronous logging is enabled by default if using Viper configuration. To enable it explicitly:

```go
// In your application configuration (e.g., config.yaml)
logger:
  async: true
  async_channel_size: 2000
```

### Adding Multiple Loggers

```go
// Add a secondary logger
stdLogger := logger.NewStdLogger(os.Stdout, "[MYAPP] ", log.LstdFlags)
logger.AddLogger(stdLogger)
```

## Configuration Options

The `Config` struct supports the following fields:

- `LogDir`: Directory where log files are stored
- `BaseName`: Base name for log files
- `Format`: Log format ("json" or "console")
- `Level`: Minimum log level to output
- `Compress`: Whether to compress old log files
- `TimeFormat`: Format for timestamps in logs
- `MaxSizeMB`: Maximum size of a log file before rotation (in MB)
- `MaxBackups`: Maximum number of log files to keep
- `MaxAgeDays`: Maximum age of log files to keep (in days)
- `EnableWarnFile`: Whether to create a separate file for warn logs
- `EnableErrorFile`: Whether to create a separate file for error logs
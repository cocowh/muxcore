// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package logger

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/spf13/viper"
)

var (
	loggers      []Logger
	once         sync.Once
	logEventPool = sync.Pool{New: func() any {
		return &LogEvent{}
	}}
)

type Logger interface {
	Debugf(format string, args ...any)
	Debug(args ...any)
	Infof(format string, args ...any)
	Info(args ...any)
	Warnf(format string, args ...any)
	Warn(args ...any)
	Errorf(format string, args ...interface{})
	Error(args ...any)
	Fatalf(format string, args ...interface{})
	Fatal(args ...any)
}

type LogEvent struct {
	Level   Level
	Format  string
	Args    []any
	IsFatal bool
}

func acquireLogEvent(level Level, format string, args ...any) *LogEvent {
	logEvent := logEventPool.Get().(*LogEvent)
	logEvent.Level = level
	logEvent.Format = format
	logEvent.Args = args
	logEvent.IsFatal = level == FatalLevel
	return logEvent
}

func releaseLogEvent(event *LogEvent) {
	event.Level = TraceLevel
	event.Format = ""
	event.Args = nil
	event.IsFatal = false
	logEventPool.Put(event)
}

// InitDefaultLogger initialize the logger
func InitDefaultLogger(config *Config) error {
	if config == nil {
		config = &Config{
			LogDir:          viper.GetString("logger.log_dir"),
			BaseName:        viper.GetString("logger.base_name"),
			Format:          viper.GetString("logger.format"),
			Level:           Level(viper.GetInt("logger.level")),
			Compress:        viper.GetBool("logger.compress"),
			TimeFormat:      viper.GetString("logger.time_format"),
			MaxSizeMB:       viper.GetInt("logger.max_size_mb"),
			MaxAgeDays:      viper.GetInt("logger.max_age_days"),
			MaxBackups:      viper.GetInt("logger.max_backups"),
			EnableWarnFile:  viper.GetBool("logger.enable_warn_file"),
			EnableErrorFile: viper.GetBool("logger.enable_error_file"),
		}
	}

	isAsyncLogger := viper.GetBool("logger.async")

	asyncChanSize := viper.GetInt("logger.async_channel_size")

	baseLogger, err := NewZapLoggerWithConfig(config)
	if err != nil {
		return err
	}

	var logger Logger = baseLogger
	if isAsyncLogger {
		logger = NewAsyncLogger(baseLogger, asyncChanSize)

		go func() {
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			<-sigCh

			if asyncLogger, ok := logger.(*AsyncLogger); ok {
				asyncLogger.Stop()
			}
		}()
	}

	AddLogger(logger)
	return nil
}

type AsyncLogger struct {
	backend  Logger
	channel  chan *LogEvent
	wg       sync.WaitGroup
	stopChan chan struct{}
}

func NewAsyncLogger(backend Logger, bufferSize int) Logger {
	if bufferSize <= 0 {
		bufferSize = 1000
	}

	logger := &AsyncLogger{
		backend:  backend,
		channel:  make(chan *LogEvent, bufferSize),
		stopChan: make(chan struct{}),
	}

	logger.wg.Add(1)
	go logger.processEvents()

	return logger
}

func (l *AsyncLogger) processEvents() {
	defer l.wg.Done()
	for {
		select {
		case event := <-l.channel:
			l.handleEvent(event)
		case <-l.stopChan:
			l.flushEvents()
			return
		}
	}
}

func (l *AsyncLogger) handleEvent(event *LogEvent) {
	if event == nil {
		return
	}

	defer releaseLogEvent(event)

	switch event.Level {
	case DebugLevel:
		if event.Format != "" {
			l.backend.Debugf(event.Format, event.Args...)
		} else {
			l.backend.Debug(event.Args...)
		}
	case InfoLevel:
		if event.Format != "" {
			l.backend.Infof(event.Format, event.Args...)
		} else {
			l.backend.Info(event.Args...)
		}
	case WarnLevel:
		if event.Format != "" {
			l.backend.Warnf(event.Format, event.Args...)
		} else {
			l.backend.Warn(event.Args...)
		}
	case ErrorLevel:
		if event.Format != "" {
			l.backend.Errorf(event.Format, event.Args...)
		} else {
			l.backend.Error(event.Args...)
		}
	case FatalLevel:
		if event.Format != "" {
			l.backend.Fatalf(event.Format, event.Args...)
		} else {
			l.backend.Fatal(event.Args...)
		}
	}
}

func (l *AsyncLogger) flushEvents() {
	for {
		select {
		case event := <-l.channel:
			l.handleEvent(event)
		default:
			return
		}
	}
}

func (l *AsyncLogger) Stop() {
	close(l.stopChan)
	l.wg.Wait()
}

func (l *AsyncLogger) Debugf(format string, args ...any) {
	l.channel <- acquireLogEvent(DebugLevel, format, args...)
}

func (l *AsyncLogger) Debug(args ...any) {
	l.channel <- acquireLogEvent(DebugLevel, "", args...)
}

func (l *AsyncLogger) Infof(format string, args ...any) {
	l.channel <- acquireLogEvent(InfoLevel, format, args...)
}

func (l *AsyncLogger) Info(args ...any) {
	l.channel <- acquireLogEvent(InfoLevel, "", args...)
}

func (l *AsyncLogger) Warnf(format string, args ...any) {
	l.channel <- acquireLogEvent(WarnLevel, format, args...)
}

func (l *AsyncLogger) Warn(args ...any) {
	l.channel <- acquireLogEvent(WarnLevel, "", args...)
}

func (l *AsyncLogger) Errorf(format string, args ...any) {
	l.channel <- acquireLogEvent(ErrorLevel, format, args...)
}

func (l *AsyncLogger) Error(args ...any) {
	l.channel <- acquireLogEvent(ErrorLevel, "", args...)
}

func (l *AsyncLogger) Fatalf(format string, args ...any) {
	l.channel <- acquireLogEvent(FatalLevel, format, args...)
	l.flushEvents()
}

func (l *AsyncLogger) Fatal(args ...any) {
	l.channel <- acquireLogEvent(FatalLevel, "", args...)
	l.flushEvents()
}

func Debugf(msg string, fields ...any) {
	if len(loggers) == 0 {
		fmt.Printf(msg, fields...)
	} else {
		for _, logger := range loggers {
			logger.Debugf(msg, fields...)
		}
	}
}

func Debug(fields ...any) {
	if len(loggers) == 0 {
		fmt.Println(fields...)
	} else {
		for _, logger := range loggers {
			logger.Debug(fields...)
		}
	}
}

func Infof(msg string, fields ...any) {
	if len(loggers) == 0 {
		fmt.Printf(msg, fields...)
	} else {
		for _, logger := range loggers {
			logger.Infof(msg, fields...)
		}
	}
}

func Info(fields ...any) {
	if len(loggers) == 0 {
		fmt.Println(fields...)
	} else {
		for _, logger := range loggers {
			logger.Info(fields...)
		}
	}
}

func Warnf(msg string, fields ...any) {
	if len(loggers) == 0 {
		fmt.Printf(msg, fields...)
	} else {
		for _, logger := range loggers {
			logger.Warnf(msg, fields...)
		}
	}
}

func Warn(fields ...any) {
	if len(loggers) == 0 {
		fmt.Println(fields...)
	} else {
		for _, logger := range loggers {
			logger.Warn(fields...)
		}
	}
}

func Errorf(msg string, fields ...any) {
	if len(loggers) == 0 {
		fmt.Printf(msg, fields...)
	} else {
		for _, logger := range loggers {
			logger.Errorf(msg, fields...)
		}
	}
}

func Error(fields ...any) {
	if len(loggers) == 0 {
		fmt.Println(fields...)
	} else {
		for _, logger := range loggers {
			logger.Error(fields...)
		}
	}
}

func Fatalf(msg string, fields ...any) {
	if len(loggers) == 9 {
		fmt.Printf(msg, fields...)
	} else {
		for _, logger := range loggers {
			logger.Fatalf(msg, fields...)
		}
	}
	os.Exit(1)
}

func Fatal(fields ...any) {
	if len(loggers) == 0 {
		fmt.Println(fields...)
	} else {
		for _, logger := range loggers {
			logger.Fatal(fields...)
		}
	}
	os.Exit(1)
}

func AddLogger(logger ...Logger) {
	if logger != nil {
		once.Do(func() {
			loggers = append(loggers, logger...)
		})
	}
}

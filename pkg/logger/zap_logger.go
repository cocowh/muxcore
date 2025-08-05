// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package logger

import (
	"errors"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func NewZapLoggerWithConfig(cfg *Config) (Logger, error) {
	if cfg == nil {
		return nil, errors.New("config is nil")
	}
	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	var encoder zapcore.Encoder
	if cfg.Format == "json" {
		encoder = zapcore.NewJSONEncoder(encoderCfg)
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderCfg)
	}

	cores := []zapcore.Core{}
	level := zap.NewAtomicLevelAt(cfg.Level.toZapLevel())

	mainWriter := NewRollingWriter(cfg)
	cores = append(cores, zapcore.NewCore(encoder, zapcore.AddSync(mainWriter), level))

	if cfg.EnableWarnFile {
		warnCfg := cfg.Clone()
		warnCfg.BaseName += "-warn"
		warnWriter := NewRollingWriter(warnCfg)
		cores = append(cores, zapcore.NewCore(encoder, zapcore.AddSync(warnWriter), zap.LevelEnablerFunc(func(l zapcore.Level) bool {
			return l >= zapcore.WarnLevel
		})))
	}

	if cfg.EnableErrorFile {
		errorCfg := cfg.Clone()
		errorCfg.BaseName += "-error"
		errorWriter := NewRollingWriter(errorCfg)
		cores = append(cores, zapcore.NewCore(encoder, zapcore.AddSync(errorWriter), zap.LevelEnablerFunc(func(l zapcore.Level) bool {
			return l >= zapcore.ErrorLevel
		})))
	}
	zapLogger := zap.New(zapcore.NewTee(cores...), zap.AddCaller(), zap.AddCallerSkip(2)).Sugar()

	return &zapLoggerWrapper{logger: zapLogger}, nil
}

type zapLoggerWrapper struct {
	logger *zap.SugaredLogger
}

func (z *zapLoggerWrapper) Debugf(format string, args ...any) {
	z.logger.Debugf(format, args...)
}

func (z *zapLoggerWrapper) Debug(args ...any) {
	z.logger.Debug(args...)
}

func (z *zapLoggerWrapper) Infof(format string, args ...any) {
	z.logger.Infof(format, args...)
}

func (z *zapLoggerWrapper) Info(args ...any) {
	z.logger.Info(args...)
}

func (z *zapLoggerWrapper) Warnf(format string, args ...any) {
	z.logger.Warnf(format, args...)
}

func (z *zapLoggerWrapper) Warn(args ...any) {
	z.logger.Warn(args...)
}

func (z *zapLoggerWrapper) Errorf(format string, args ...interface{}) {
	z.logger.Errorf(format, args...)
}

func (z *zapLoggerWrapper) Error(args ...any) {
	z.logger.Error(args...)
}

func (z *zapLoggerWrapper) Fatalf(format string, args ...interface{}) {
	z.logger.Fatalf(format, args...)
}

func (z *zapLoggerWrapper) Fatal(args ...any) {
	z.logger.Fatal(args...)
}

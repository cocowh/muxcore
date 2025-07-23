// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"os"
)

func NewZapLoggerWithConfig(cfg *Config) (Logger, error) {
	level := zap.NewAtomicLevelAt(cfg.Level.toZapLevel())
	var ws zapcore.WriteSyncer
	if cfg.Output == "stdout" {
		ws = zapcore.AddSync(os.Stdout)
	} else {
		lj := &lumberjack.Logger{
			Filename:   cfg.Output,
			MaxSize:    cfg.RotateConfig.MaxSize,
			MaxAge:     cfg.RotateConfig.MaxAge,
			MaxBackups: cfg.RotateConfig.MaxBackups,
			Compress:   cfg.RotateConfig.Compress,
		}
		ws = zapcore.AddSync(lj)
	}
	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	var encoder zapcore.Encoder
	if cfg.Format == "json" {
		encoder = zapcore.NewJSONEncoder(encoderCfg)
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderCfg)
	}
	core := zapcore.NewCore(encoder, ws, level)
	zapLogger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(2))
	return &zapLoggerWrapper{logger: zapLogger.Sugar()}, nil
}

type zapLoggerWrapper struct {
	logger *zap.SugaredLogger
}

func (z *zapLoggerWrapper) Debugf(format string, args ...any) {
	z.logger.Debugf(format, args...)
}

func (z *zapLoggerWrapper) Debugln(args ...any) {
	z.logger.Debug(args...)
}

func (z *zapLoggerWrapper) Infof(format string, args ...any) {
	z.logger.Infof(format, args...)
}

func (z *zapLoggerWrapper) Infoln(args ...any) {
	z.logger.Info(args...)
}

func (z *zapLoggerWrapper) Warnf(format string, args ...any) {
	z.logger.Warnf(format, args...)
}

func (z *zapLoggerWrapper) Warnln(args ...any) {
	z.logger.Warn(args...)
}

func (z *zapLoggerWrapper) Errorf(format string, args ...interface{}) {
	z.logger.Errorf(format, args...)
}

func (z *zapLoggerWrapper) Errorln(args ...any) {
	z.logger.Error(args...)
}

func (z *zapLoggerWrapper) Fatalf(format string, args ...interface{}) {
	z.logger.Fatalf(format, args...)
}

func (z *zapLoggerWrapper) Fatalln(args ...any) {
	z.logger.Fatal(args...)
}

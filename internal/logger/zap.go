package logger

import (
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// L is the global logger instance
var L *zap.Logger

func init() {
	config := zap.NewProductionConfig()
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	var err error
	L, err = config.Build()
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize zap logger: %v", err))
	}
}

// Sync flushes any buffered log entries and should be called before application exit
func Sync() {
	if err := L.Sync(); err != nil {
		L.Error("Failed to sync logger",
			zap.Error(err),
		)
	}
}

// Info logs a message at InfoLevel
func Info(msg string, fields ...zap.Field) {
	L.WithOptions(zap.AddCallerSkip(1)).Info(msg, fields...)
}

// Debug logs a message at DebugLevel
func Debug(msg string, fields ...zap.Field) {
	L.WithOptions(zap.AddCallerSkip(1)).Debug(msg, fields...)
}

// Error logs a message at ErrorLevel
func Error(msg string, fields ...zap.Field) {
	L.WithOptions(zap.AddCallerSkip(1)).Error(msg, fields...)
}

// Warn logs a message at WarnLevel
func Warn(msg string, fields ...zap.Field) {
	L.WithOptions(zap.AddCallerSkip(1)).Warn(msg, fields...)
}

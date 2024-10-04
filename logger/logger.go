package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var logger *zap.Logger

func InitializeDefault() {
	logger = zap.Must(zap.NewProduction())
}

func SetLogLevel(appEnv string, logLevel string) {
	var err error
	var zapConfig zap.Config

	if appEnv == "development" {
		zapConfig = zap.NewDevelopmentConfig()
	} else {
		zapConfig = zap.NewProductionConfig()
	}

	switch logLevel {
	case "error":
		zapConfig.Level = zap.NewAtomicLevelAt(zap.ErrorLevel)
	case "warn":
		zapConfig.Level = zap.NewAtomicLevelAt(zap.WarnLevel)
	case "debug":
		zapConfig.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	default:
		zapConfig.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}

	logger, err = zapConfig.Build(zap.AddCallerSkip(1))
	if err != nil {
		panic(err)
	}
}

func Info(message string, fields ...zapcore.Field) {
	logger.Info(message, fields...)
}

func Warn(message string, fields ...zapcore.Field) {
	logger.Warn(message, fields...)
}

func Debug(message string, fields ...zapcore.Field) {
	logger.Debug(message, fields...)
}

func Error(message string, fields ...zapcore.Field) {
	logger.Error(message, fields...)
}

func Fatal(message string, fields ...zapcore.Field) {
	logger.Fatal(message, fields...)
}

func Sync() {
	// ignore error
	_ = logger.Sync()
}

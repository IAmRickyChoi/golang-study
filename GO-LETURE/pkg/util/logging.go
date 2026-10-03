package util

import (
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var logger *zap.Logger

func InitZapLogger(cfg Config) {
	if err := os.MkdirAll(cfg.Directory, 0o755); err != nil {
		panic(err)
	}

	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
	encoder := zapcore.NewConsoleEncoder(encoderConfig)

	debugWriter := zapcore.AddSync(&lumberjack.Logger{
		Filename:   filepath.Join(cfg.Directory, cfg.Files.Debug),
		MaxSize:    cfg.Rotation.MaxSize,
		MaxBackups: cfg.Rotation.MaxBackups,
		MaxAge:     cfg.Rotation.MaxAge,
		Compress:   cfg.Rotation.Compress,
	})

	infoWriter := zapcore.AddSync(&lumberjack.Logger{
		Filename:   filepath.Join(cfg.Directory, cfg.Files.Info),
		MaxSize:    cfg.Rotation.MaxSize,
		MaxBackups: cfg.Rotation.MaxBackups,
		MaxAge:     cfg.Rotation.MaxAge,
		Compress:   cfg.Rotation.Compress,
	})

	errorWriter := zapcore.AddSync(&lumberjack.Logger{
		Filename:   filepath.Join(cfg.Directory, cfg.Files.Error),
		MaxSize:    cfg.Rotation.MaxSize,
		MaxBackups: cfg.Rotation.MaxBackups,
		MaxAge:     cfg.Rotation.MaxAge,
		Compress:   cfg.Rotation.Compress,
	})

	consoleWriter := zapcore.Lock(os.Stdout)

	debugLevel := zap.LevelEnablerFunc(func(level zapcore.Level) bool {
		return level == zapcore.DebugLevel
	})

	infoLevel := zap.LevelEnablerFunc(func(level zapcore.Level) bool {
		return level == zapcore.InfoLevel || level == zap.WarnLevel
	})

	errorLevel := zap.LevelEnablerFunc(func(level zapcore.Level) bool {
		return level >= zapcore.ErrorLevel
	})

	core := zapcore.NewTee(
		zapcore.NewCore(encoder, debugWriter, debugLevel),
		zapcore.NewCore(encoder, infoWriter, infoLevel),
		zapcore.NewCore(encoder, errorWriter, errorLevel),
		zapcore.NewCore(encoder, consoleWriter, zap.LevelEnablerFunc(func(level zapcore.Level) bool {
			return level >= zapcore.DebugLevel
		})),
	)

	logger = zap.New(core, zap.AddCaller())
	zap.ReplaceGlobals(logger)
}

func Sync() error {
	if logger == nil {
		return nil
	}
	return logger.Sync()
}

func Info(message string, fields ...zap.Field) {
	zap.L().Info(message, fields...)
}

func Debug(message string, fields ...zap.Field) {
	zap.L().Debug(message, fields...)
}
func Error(message string, fields ...zap.Field) {
	zap.L().Error(message, fields...)
}

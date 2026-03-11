package zap

import (
	"os"

	"go.elastic.co/ecszap"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func getLogWriter(filepath string) zapcore.WriteSyncer {
	file, _ := os.Create(filepath)
	return zapcore.AddSync(file)
}

// GetLogger ...
func GetLogger() *zap.SugaredLogger {
	cfg := zap.NewProductionConfig()

	cfg.Encoding = "json"
	l, _ := cfg.Build()
	return l.Sugar()
}

// GetConsoleLogger ...
func GetConsoleLogger() *zap.SugaredLogger {
	encoder := ecszap.NewDefaultEncoderConfig()
	core := ecszap.NewCore(encoder, os.Stdout, zapcore.DebugLevel)
	logger := zap.New(core, zap.AddCaller())
	return logger.Sugar()
}

// GetFileLogger ...
func GetFileLogger(filepath string) *zap.SugaredLogger {
	writerSyncer := getLogWriter(filepath)
	encoder := ecszap.NewDefaultEncoderConfig()

	core := ecszap.NewCore(encoder, writerSyncer, zapcore.DebugLevel)

	logger := zap.New(core, zap.AddCaller())
	return logger.Sugar()
}

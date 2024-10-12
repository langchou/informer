package mylog

import (
	"log"

	"github.com/natefinch/lumberjack"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var logger *Logger // 声明全局 logger

type Logger struct {
	Sugar *zap.SugaredLogger
}

func InitLogger(logFile string, maxSize, maxBackups, maxAge int, compress bool) {
	// 使用 lumberjack 进行日志轮转配置
	lumberjackLogger := &lumberjack.Logger{
		Filename:   logFile,
		MaxSize:    maxSize,    // MB
		MaxBackups: maxBackups, // 备份的最大数量
		MaxAge:     maxAge,     // 天数
		Compress:   compress,   // 是否压缩旧日志
	}

	// 创建 zap 核心配置
	writeSyncer := zapcore.AddSync(lumberjackLogger)
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "time"
	encoderConfig.LevelKey = "level"
	encoderConfig.MessageKey = "msg"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	core := zapcore.NewCore(zapcore.NewConsoleEncoder(encoderConfig), writeSyncer, zap.InfoLevel)
	zapLogger := zap.New(core)

	// 初始化全局 logger
	logger = &Logger{Sugar: zapLogger.Sugar()}
}

// 全局 Info 方法
func Info(format string, args ...interface{}) {
	if logger == nil {
		log.Fatalf("Logger not initialized. Call InitLogger first.")
	}
	logger.Sugar.Infof(format, args...)
}

// 全局 Error 方法
func Error(format string, args ...interface{}) {
	if logger == nil {
		log.Fatalf("Logger not initialized. Call InitLogger first.")
	}
	logger.Sugar.Errorf(format, args...)
}

// 同步日志
func Sync() {
	if logger != nil {
		err := logger.Sugar.Sync()
		if err != nil {
			log.Printf("Error syncing logger: %v", err)
		}
	}
}

package log

import (
	"log"

	"github.com/natefinch/lumberjack"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Logger struct {
	Sugar *zap.SugaredLogger
}

func InitLogger(logFile string, maxSize, maxBackups, maxAge int, compress bool) *Logger {
	// 使用 lumberjack 进行日志轮转配置
	logger := &lumberjack.Logger{
		Filename:   logFile,
		MaxSize:    maxSize,    // MB
		MaxBackups: maxBackups, // 备份的最大数量
		MaxAge:     maxAge,     // 天数
		Compress:   compress,   // 是否压缩旧日志
	}

	// 创建 zap 核心配置
	writeSyncer := zapcore.AddSync(logger)
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "time"
	encoderConfig.LevelKey = "level"
	encoderConfig.MessageKey = "msg"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	core := zapcore.NewCore(zapcore.NewConsoleEncoder(encoderConfig), writeSyncer, zap.InfoLevel)
	zapLogger := zap.New(core)

	return &Logger{Sugar: zapLogger.Sugar()}
}

func (l *Logger) Info(msg string, keysAndValues ...interface{}) {
	l.Sugar.Infow(msg, keysAndValues...)
}

func (l *Logger) Error(msg string, keysAndValues ...interface{}) {
	l.Sugar.Errorw(msg, keysAndValues...)
}

func (l *Logger) Sync() {
	err := l.Sugar.Sync()
	if err != nil {
		log.Printf("Error syncing logger: %v", err)
	}
}

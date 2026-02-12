package logger

import (
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"mbsscaner/config"
)

// NewLogger 创建新的日志记录器
func NewLogger(config config.LoggerConfig) (*zap.Logger, error) {
	// 解析日志级别
	level, err := parseLogLevel(config.Level)
	if err != nil {
		return nil, err
	}

	// 创建编码器配置
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "message",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// 根据输出类型创建不同的核心
	var cores []zapcore.Core

	switch strings.ToLower(config.Output) {
	case "stdout", "console":
		// 控制台输出
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoder := zapcore.NewConsoleEncoder(encoderConfig)
		cores = append(cores, zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), level))

	case "file":
		// 文件输出
		if config.Filename == "" {
			config.Filename = "logs/mbsscaner.log"
		}

		// 创建日志目录
		if err := os.MkdirAll(getLogDir(config.Filename), 0755); err != nil {
			return nil, err
		}

		// 配置滚动日志
		lumberJackLogger := &lumberjack.Logger{
			Filename:   config.Filename,
			MaxSize:    config.MaxSize,    // MB
			MaxBackups: config.MaxBackups, // 保留的旧日志文件数量
			MaxAge:     config.MaxAge,     // 保留日志文件的最大天数
			Compress:   config.Compress,   // 是否压缩旧日志文件
		}

		encoder := zapcore.NewJSONEncoder(encoderConfig)
		cores = append(cores, zapcore.NewCore(encoder, zapcore.AddSync(lumberJackLogger), level))

	case "both":
		// 同时输出到控制台和文件
		if config.Filename == "" {
			config.Filename = "logs/mbsscaner.log"
		}

		// 创建日志目录
		if err := os.MkdirAll(getLogDir(config.Filename), 0755); err != nil {
			return nil, err
		}

		// 控制台输出
		consoleEncoderConfig := encoderConfig
		consoleEncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		consoleEncoder := zapcore.NewConsoleEncoder(consoleEncoderConfig)
		cores = append(cores, zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), level))

		// 文件输出
		lumberJackLogger := &lumberjack.Logger{
			Filename:   config.Filename,
			MaxSize:    config.MaxSize,
			MaxBackups: config.MaxBackups,
			MaxAge:     config.MaxAge,
			Compress:   config.Compress,
		}

		fileEncoder := zapcore.NewJSONEncoder(encoderConfig)
		cores = append(cores, zapcore.NewCore(fileEncoder, zapcore.AddSync(lumberJackLogger), level))

	default:
		// 默认输出到控制台
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoder := zapcore.NewConsoleEncoder(encoderConfig)
		cores = append(cores, zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), level))
	}

	// 如果输出到文件或both，同时输出错误级别日志到stderr
	if strings.ToLower(config.Output) == "file" || strings.ToLower(config.Output) == "both" {
		errorEncoder := zapcore.NewJSONEncoder(encoderConfig)
		errorCore := zapcore.NewCore(errorEncoder, zapcore.AddSync(os.Stderr), zapcore.ErrorLevel)
		cores = append(cores, errorCore)
	}

	// 创建核心
	core := zapcore.NewTee(cores...)

	// 创建日志记录器
	logger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))

	return logger, nil
}

// parseLogLevel 解析日志级别
func parseLogLevel(level string) (zapcore.Level, error) {
	switch strings.ToLower(level) {
	case "debug":
		return zapcore.DebugLevel, nil
	case "info":
		return zapcore.InfoLevel, nil
	case "warn", "warning":
		return zapcore.WarnLevel, nil
	case "error":
		return zapcore.ErrorLevel, nil
	case "fatal":
		return zapcore.FatalLevel, nil
	case "panic":
		return zapcore.PanicLevel, nil
	default:
		return zapcore.InfoLevel, nil
	}
}

// getLogDir 获取日志目录
func getLogDir(filename string) string {
	lastSlash := strings.LastIndex(filename, "/")
	if lastSlash == -1 {
		return "."
	}
	return filename[:lastSlash]
}

// NewDevelopmentLogger 创建开发环境日志记录器
func NewDevelopmentLogger() (*zap.Logger, error) {
	config := zap.NewDevelopmentConfig()
	config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	return config.Build()
}

// NewProductionLogger 创建生产环境日志记录器
func NewProductionLogger(filename string) (*zap.Logger, error) {
	if filename == "" {
		filename = "logs/mbsscaner.log"
	}

	// 创建日志目录
	if err := os.MkdirAll(getLogDir(filename), 0755); err != nil {
		return nil, err
	}

	// 配置生产环境日志
	config := zap.NewProductionConfig()
	config.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	config.OutputPaths = []string{filename}
	config.ErrorOutputPaths = []string{"stderr"}
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	// 使用lumberjack进行日志滚动
	lumberJackLogger := &lumberjack.Logger{
		Filename:   filename,
		MaxSize:    100, // MB
		MaxBackups: 3,
		MaxAge:     28, // days
		Compress:   true,
	}

	encoder := zapcore.NewJSONEncoder(config.EncoderConfig)
	core := zapcore.NewCore(encoder, zapcore.AddSync(lumberJackLogger), config.Level.Level())

	return zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel)), nil
}

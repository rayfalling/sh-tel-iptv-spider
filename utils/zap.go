package utils

import (
	"iptv-spider-sh/config"
	"iptv-spider-sh/global"
	"os"
	"path"
	"time"

	zaprotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"go.uber.org/zap/zapcore"
)

func GetWriteSyncer() (zapcore.WriteSyncer, error) {
	fileWriter, err := zaprotatelogs.New(
		path.Join(global.CONFIG.Zap.Director, "%Y-%m-%d.log"),
		zaprotatelogs.WithLinkName(global.CONFIG.Zap.LinkName),
		zaprotatelogs.WithMaxAge(7*24*time.Hour),
		zaprotatelogs.WithRotationTime(24*time.Hour),
	)
	if err != nil {
		// 必须显式返回 nil：AddSync 一个类型化 nil 指针会得到非 nil 的 WriteSyncer，
		// 调用方判空失效，写日志时才会空指针 panic。
		return nil, err
	}
	if global.CONFIG.Zap.LogInConsole {
		return zapcore.NewMultiWriteSyncer(zapcore.AddSync(os.Stdout), zapcore.AddSync(fileWriter)), nil
	}
	return zapcore.AddSync(fileWriter), nil
}

func GetAccessLogWriteSyncer(cfg config.AccessLog) (zapcore.WriteSyncer, error) {
	maxAge := 7 * 24 * time.Hour
	if cfg.MaxAge > 0 {
		maxAge = time.Duration(cfg.MaxAge) * 24 * time.Hour
	}
	rotationTime := 24 * time.Hour
	if cfg.RotationTime > 0 {
		rotationTime = time.Duration(cfg.RotationTime) * time.Hour
	}

	fileWriter, err := zaprotatelogs.New(
		path.Join(cfg.Director, "%Y-%m-%d.log"),
		zaprotatelogs.WithMaxAge(maxAge),
		zaprotatelogs.WithRotationTime(rotationTime),
	)
	if err != nil {
		return nil, err
	}
	return zapcore.AddSync(fileWriter), nil
}

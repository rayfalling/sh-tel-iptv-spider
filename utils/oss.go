package utils

import (
	"bytes"
	"context"
	"fmt"
	"iptv-spider-sh/global"
	"os"
	"path"
	"strings"

	"github.com/minio/minio-go/v7"
	"go.uber.org/zap"
)

func UploadToOSS(key string, data []byte) {
	if global.COS != nil {
		r := bytes.NewReader(data)
		_, err := global.COS.Object.Put(context.Background(), key, r, nil)
		if err != nil {
			// 原实现在这里 panic(err)：上传定时任务运行在 cron 自己的 goroutine 里，
			// 一次网络抖动 / 密钥过期就会 panic 掉整个进程。这里改为记录日志后返回。
			global.LOG.Error("上传到 COS 失败", zap.String("key", key), zap.Error(err))
		}
	} else if global.MinioClient != nil {
		bucket := global.CONFIG.OSS.Bucket
		r := bytes.NewReader(data)
		if after, ok := strings.CutPrefix(key, "/"); ok {
			key = after
		}
		var opts minio.PutObjectOptions
		if strings.HasSuffix(key, ".xml") {
			opts.ContentType = "application/xml"
		}
		_, err := global.MinioClient.PutObject(context.Background(), bucket, key, r, int64(len(data)), opts)
		if err != nil {
			global.LOG.Error("上传到存储服务失败", zap.Error(err))
		}
	} else {
		global.LOG.Error("未配置存储服务")
	}
}

func SaveToLogDir(data []byte, filename string) {
	logDir := fmt.Sprintf("%s/tv", global.CONFIG.Zap.Director)
	if ok, _ := PathExists(logDir); !ok {
		os.MkdirAll(logDir, os.ModePerm)
	}
	filePath := path.Join(logDir, filename)
	os.WriteFile(filePath, data, 0644)
}

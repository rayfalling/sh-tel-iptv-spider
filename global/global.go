package global

import (
	"strconv"
	"sync/atomic"
	"time"

	"github.com/astaxie/beego/cache"
	"github.com/minio/minio-go/v7"
	"github.com/robfig/cron/v3"
	"github.com/spf13/viper"
	"github.com/tencentyun/cos-go-sdk-v5"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"

	"iptv-spider-sh/config"
)

var (
	DB                 *gorm.DB
	CACHE              cache.Cache
	COS                *cos.Client
	MinioClient        *minio.Client
	VIPER              *viper.Viper
	CONFIG             *config.Server
	LOG                *zap.Logger
	ACCESS_LOG         *zap.Logger
	CRON               *cron.Cron
	ConcurrencyControl = &singleflight.Group{}

	// VERSION 版本号：可由 -ldflags "-X main.version=xxx" 注入，
	// 未注入时 main 会尝试从二进制的 VCS 构建信息推断。
	VERSION = "dev"

	// StartedAt 进程启动时间，用于 /api/health 计算运行时长
	StartedAt = time.Now()

	// m3uCacheEpoch M3U 相关缓存的版本号。
	// 频道显隐/重命名/排序等操作会让它自增，从而让旧缓存键自然失效，
	// 无需枚举并逐个删除缓存条目（beego cache 不提供键枚举）。
	m3uCacheEpoch int64
)

// BumpM3UCache 使全部 M3U 相关缓存立即失效
func BumpM3UCache() {
	atomic.AddInt64(&m3uCacheEpoch, 1)
}

// M3UCacheVersion 当前缓存版本号（拼进 M3U 缓存键）
func M3UCacheVersion() string {
	return strconv.FormatInt(atomic.LoadInt64(&m3uCacheEpoch), 10)
}

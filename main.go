package main

import (
	"runtime/debug"
	"strings"

	"iptv-spider-sh/global"
	"iptv-spider-sh/initialize"
	"iptv-spider-sh/modules/auth"
	"iptv-spider-sh/modules/cronmgr"
	"iptv-spider-sh/modules/settings"
	"iptv-spider-sh/router"
	"iptv-spider-sh/utils"

	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/middleware/recover"
)

// version 版本号，可由 -ldflags "-X main.version=xxx" 注入
var version = "dev"

func main() {
	initialize.Viper()
	global.VERSION = resolveVersion()

	global.LOG = initialize.Zap()
	global.ACCESS_LOG = initialize.AccessLog()
	global.CACHE = initialize.Cache()
	global.DB = initialize.Gorm()
	ossType := global.CONFIG.System.OSSType
	if ossType == "cos" {
		global.COS = initialize.COS()
	} else if ossType == "minio" {
		global.MinioClient = initialize.Minio()
	}

	initialize.InitCron()

	if global.DB != nil {
		initialize.MysqlTables(global.DB) // 初始化表
		// EPG 配置：首次用 config.yaml 落库，之后以数据库为准（可在管理面板在线改）
		settings.Load()
		// 程序结束前关闭数据库链接
		if sqlDB, dbErr := global.DB.DB(); dbErr == nil && sqlDB != nil {
			defer sqlDB.Close()
		}
	} else {
		// 数据库不可用时不能继续：认证信息读写、频道/EPG 查询都会落在 nil *gorm.DB 上，
		// 触发空指针 panic（gorm 的方法会解引用 receiver）。这里保持 HTTP 可用，
		// 只跳过依赖数据库的后台任务，便于通过 /api/health 与日志定位问题。
		global.LOG.Error("数据库初始化失败：跳过建表、EPG 配置加载、频道抓取与定时任务")
	}

	app := iris.New()

	app.Use(recover.New())

	router.InitRouters(app)

	stb := global.CONFIG.Stb
	if global.DB == nil {
		// 没有数据库时认证流程无法读写会话，直接跳过（NewGlobalClient 内部也会检查）
		global.LOG.Error("数据库未就绪，跳过认证客户端与定时任务初始化")
	} else if client, err := auth.NewGlobalClient(stb.UID, stb.SN, stb.MAC, stb.IP); err != nil {
		// 注意：这里不再直接 return 退出。
		// 原先认证参数不对时进程会立刻结束（退出码 0），管理面板/健康检查也一起没了，
		// 只能靠翻日志排查。现在保持 HTTP 服务可用（/api/health 会显示 degraded），
		// 便于通过面板定位问题；抓取任务会因客户端为空而跳过。
		global.LOG.Error("New AuthClient 失败，抓取功能不可用（HTTP 接口仍可用，请检查 stb 参数与专网连通性）: " + err.Error())
	} else {
		cronmgr.RegisterAll(client)

		// 启动一个协程来运行（SafeGo 内部 recover：启动抓取失败不应终止进程）
		utils.SafeGo("startup-fetch", func() {
			// 启动时获取频道列表
			client.FetchChannelList()
			// 拉取一次节目单，如果近期更新过，则不会实际运行
			client.FetchChannelProg(false)
			auth.GenerateAndUploadDiyp("")
		})
	}

	app.ConfigureHost(configHost)
	_ = app.Run(iris.Addr(global.CONFIG.System.Addr), iris.WithoutServerError(iris.ErrServerClosed))
}

// resolveVersion 版本号：优先 -ldflags 注入，其次从二进制的 VCS 构建信息推断
func resolveVersion() string {
	if v := strings.TrimSpace(version); v != "" && v != "dev" {
		return v
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		var rev string
		for _, s := range bi.Settings {
			if s.Key == "vcs.revision" {
				rev = s.Value
			}
		}
		if rev != "" {
			if len(rev) > 7 {
				rev = rev[:7]
			}
			return "dev+" + rev
		}
	}
	return "dev"
}

func configHost(su *iris.Supervisor) {
	su.RegisterOnShutdown(func() {
		println("Server Closed")
	})
}

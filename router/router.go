package router

import (
	"iptv-spider-sh/logo"
	"iptv-spider-sh/middleware"
	"iptv-spider-sh/router/api"

	"github.com/kataras/iris/v12"
)

func InitRouters(app *iris.Application) {
	registerMacros(app)

	app.Use(middleware.AccessLog)

	app.Get("/", index)

	// 内嵌台标静态文件：/logo/<文件名>.png
	// M3U 与 XMLTV 中的 tvg-logo 一直指向这个路径，但此前没有任何静态服务，全部 404。
	app.Get("/logo/{name:string}", serveLogo)

	apiRouterGroup := app.Party("/api")
	{
		api.InitApiRouters(apiRouterGroup)
	}
}

// serveLogo 输出内嵌的频道台标
func serveLogo(ctx iris.Context) {
	name := ctx.Params().Get("name")
	data, err := logo.FS.ReadFile(name)
	if err != nil {
		ctx.StatusCode(iris.StatusNotFound)
		return
	}
	ctx.Header("Cache-Control", "public, max-age=86400")
	ctx.ContentType("image/png")
	ctx.Write(data)
}

func index(ctx iris.Context) {
	ctx.WriteString(`IPTV Spider API

管理面板:
  GET /api/status.html - Web 管理面板（频道管理 / EPG 配置 / 实时日志 / 网络检查）

频道列表接口:
  GET /api/diyp     - 生成diyp文件
  GET /api/m3u8     - 生成M3U8频道列表文件(含回放)
    参数: udpxy=url, scheme=(rtsp|rtp|igmp)://, xteve=false|true, all=true(含隐藏频道), ref=false|true, ku9=false|true
  GET /api/tsM3u8   - 生成直播/时移M3U8文件
    参数: udpxy, scheme, xteve, all, ref
  GET /api/channel/list - 频道管理列表
  GET /api/channel/m3u8 - 单频道M3U8(参数 name=通用频道名)

节目单接口:
  GET /api/epg      - 生成XMLTV节目单
    参数: daysAgo(默认1), ref=false|true
  GET /api/epgjson  - 生成JSON格式节目单
    参数: days(默认1), ref=false|true
  GET /api/epg/config  - 读取EPG配置
  POST /api/epg/config - 修改EPG配置(cron/日志级别热更新)

频道管理接口(POST, JSON):
  /api/channel/toggle        {comm_name}
  /api/channel/rename        {comm_name, custom_name}
  /api/channel/sort          {orders:[{comm_name, sort_order}]}
  /api/channel/custom/add    {name, igmp, tvg_id?, logo?, group?}
  /api/channel/custom/update {comm_name, igmp?, tvg_id?, logo?, group?}
  /api/channel/custom/delete {comm_name}

任务管理接口:
  GET /api/schedule - 获取定时任务调度列表
  GET /api/run      - 手动触发任务执行
    参数: task(clean-ch/clean-chi/clean-epg/clean/update-chi/update-epg/upload-m3u/upload-xmltv/upload-xmltv7/upload-epgjson/upload-epgjson7)
    参数: ref(false|true)

系统接口:
  GET /api/health        - 健康检查
  GET /api/requests      - 最近访问记录
  GET /api/network-check - 外网/专网连通性检查
  GET /api/version-check - 版本检查
  GET /api/self-upgrade  - 在线升级(默认演练, 需 SPIDER_ALLOW_UPGRADE=true 且 confirm=true)
  GET /api/log/stream    - SSE 实时日志(参数 level=debug|info|warn|error)
  GET/POST /api/admin/log-level - 日志级别
`)
}

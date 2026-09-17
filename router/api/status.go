package api

import (
	"github.com/kataras/iris/v12"

	"iptv-spider-sh/web"
)

// statusPage GET /api/status.html
// 管理面板页面（内嵌单文件，无外部依赖）
func statusPage(ctx iris.Context) {
	ctx.ContentType("text/html; charset=utf-8")
	ctx.Header("Cache-Control", "no-cache")
	ctx.Write(web.StatusHTML)
}

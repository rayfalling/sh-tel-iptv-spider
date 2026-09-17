package api

import (
	"strings"

	"iptv-spider-sh/modules/settings"

	"github.com/kataras/iris/v12"
)

// jsonErr 统一错误响应：{"error": "..."}
func jsonErr(ctx iris.Context, status int, msg string) {
	ctx.StatusCode(status)
	ctx.JSON(iris.Map{"error": msg})
}

// jsonOK 统一成功响应
func jsonOK(ctx iris.Context, v interface{}) {
	ctx.JSON(v)
}

// writeAllowed 写操作准入判断：
//   - api_external_access = true ：允许任意来源调用（方便脚本/自动化）
//   - 关闭时：仅允许来自管理面板（同源 Referer/Origin）的写操作
//
// 对应 API.md 附录 C.2「API 访问控制」。
func writeAllowed(ctx iris.Context) bool {
	if settings.Current().ApiExternalAccess {
		return true
	}
	host := ctx.Host()
	for _, s := range []string{ctx.GetHeader("Referer"), ctx.GetHeader("Origin")} {
		if s == "" {
			continue
		}
		if host != "" && strings.Contains(s, host) {
			return true
		}
	}
	return false
}

// guardWrite 写接口统一入口检查；不允许时已写好响应
func guardWrite(ctx iris.Context) bool {
	if writeAllowed(ctx) {
		return true
	}
	jsonErr(ctx, iris.StatusForbidden, "写操作已被禁用（api_external_access=false，仅允许管理页面操作）")
	return false
}

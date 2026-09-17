package api

import (
	"strings"

	"iptv-spider-sh/global"
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

// cachedBytes 安全读取缓存中的 []byte。
//
// beego cache 在键不存在/已过期/被淘汰时返回 nil（redis 后端还会多一次网络往返），
// 原实现 `global.CACHE.Get(key).([]byte)` 是不带 ok 的断言：
// IsExist 与 Get 之间的 TOCTOU 窗口一旦命中就会 panic。
func cachedBytes(key string) ([]byte, bool) {
	v := global.CACHE.Get(key)
	if v == nil {
		return nil, false
	}
	b, ok := v.([]byte)
	if !ok {
		return nil, false
	}
	return b, true
}

// asBytes 安全断言 singleflight 返回值的类型（空结果也算合法，交由调用方判断）
func asBytes(v interface{}) ([]byte, bool) {
	b, ok := v.([]byte)
	if !ok {
		return nil, false
	}
	return b, true
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

// requireDB 数据库可用性检查。
// global.DB 为 nil 时，gorm 的方法会在解引用 receiver 时 panic（HTTP 层虽被
// Iris recover 兜住，但只会返回 500 并打一堆堆栈），这里直接回 503。
func requireDB(ctx iris.Context) bool {
	if global.DB == nil {
		jsonErr(ctx, iris.StatusServiceUnavailable, "数据库未就绪")
		return false
	}
	return true
}

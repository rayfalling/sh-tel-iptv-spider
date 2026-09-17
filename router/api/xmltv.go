package api

import (
	"iptv-spider-sh/global"
	"iptv-spider-sh/modules/auth"
	"iptv-spider-sh/utils"
	"strconv"
	"time"

	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/context"
)

// generateXmlTv 生成xmlTv文件 节目去重
func generateXmlTv(ctx iris.Context) {
	ref := ctx.FormValue("ref")
	daysAgo := ctx.FormValueDefault("daysAgo", "1")

	d, err := strconv.Atoi(daysAgo)
	if err != nil {
		ctx.WriteString(err.Error())
		return
	}

	// 缓存（键里带缓存版本号，EPG 配置变更后可立即失效）
	reqMD5Key := m3uCacheKey("generateXmlTv", "daysAgo="+strconv.Itoa(d))
	if ref != "true" && global.CACHE.IsExist(reqMD5Key) {
		// 存在缓存，直接返回
		ctx.ContentType(context.ContentXMLHeaderValue)
		ctx.Write(global.CACHE.Get(reqMD5Key).([]byte))
		return
	}
	// 并发时合并请求
	resp, err, _ := global.ConcurrencyControl.Do(reqMD5Key, func() (interface{}, error) {
		epgBytes, err := auth.GenerateXmlTv(d)
		if err != nil || epgBytes == nil {
			// 生成失败时不要把空结果写进缓存：否则接下来 default_timeout 分钟内
			// xteve 拿到的都是空节目单，只能靠 &ref=true 才绕得过去。
			global.LOG.Error("生成 XMLTV 失败，本次结果不缓存")
			return nil, err
		}
		timeOut := time.Duration(global.CONFIG.Cache.DefTimeOut)
		global.CACHE.Put(reqMD5Key, epgBytes, time.Minute*timeOut)
		utils.SaveToLogDir(epgBytes, "epg.xml")
		return epgBytes, nil
	})
	if err != nil || resp == nil {
		ctx.StatusCode(iris.StatusInternalServerError)
		ctx.WriteString("节目单生成失败，请查看日志")
		return
	}
	ctx.ContentType(context.ContentXMLHeaderValue)
	ctx.Write(resp.([]byte))
}

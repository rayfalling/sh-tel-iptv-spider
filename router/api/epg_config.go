package api

import (
	"iptv-spider-sh/global"
	"iptv-spider-sh/modules/cronmgr"
	"iptv-spider-sh/modules/loghub"
	"iptv-spider-sh/modules/settings"

	"github.com/kataras/iris/v12"
)

// getEpgConfig GET /api/epg/config
func getEpgConfig(ctx iris.Context) {
	row := settings.Current()
	row.LogLevel = loghub.LevelString()
	jsonOK(ctx, row)
}

// updateEpgConfig POST /api/epg/config
// 校验 → 落库 → 热生效（cron 热更新、日志级别热更新、URL 变化清 M3U 缓存）
func updateEpgConfig(ctx iris.Context) {
	if !guardWrite(ctx) {
		return
	}

	var patch settings.Patch
	if err := ctx.ReadJSON(&patch); err != nil {
		jsonErr(ctx, iris.StatusBadRequest, "参数解析失败")
		return
	}

	// cron 必须先校验再落库，否则非法表达式会被持久化
	if patch.FetchCron != nil {
		if err := cronmgr.ValidateSpec(*patch.FetchCron); err != nil {
			jsonErr(ctx, iris.StatusBadRequest, "Cron 表达式无效: "+err.Error())
			return
		}
	}
	if patch.LogLevel != nil {
		if err := loghub.ParseLevel(*patch.LogLevel); err != nil {
			jsonErr(ctx, iris.StatusBadRequest, err.Error())
			return
		}
	}

	cronChanged, err := settings.Save(patch)
	if err != nil {
		jsonErr(ctx, iris.StatusBadRequest, err.Error())
		return
	}

	if cronChanged {
		if err := cronmgr.UpdateEPGSchedule(settings.Current().FetchCron); err != nil {
			jsonErr(ctx, iris.StatusInternalServerError, "配置已保存，但定时任务更新失败: "+err.Error())
			return
		}
		global.LOG.Info("EPG 定时任务已热更新为: " + settings.Current().FetchCron)
	}

	// 影响 M3U 输出的字段变化时清缓存（通过版本号让旧键自然失效）
	if patch.RtspUrl != nil || patch.RtpUrl != nil || patch.LogoUrl != nil || patch.XmlUrl != nil {
		global.BumpM3UCache()
	}

	jsonOK(ctx, iris.Map{"success": true, "fetch_cron_changed": cronChanged})
}

package api

import (
	"strconv"
	"time"

	"iptv-spider-sh/global"
	"iptv-spider-sh/modules/auth"
	"iptv-spider-sh/utils"

	"github.com/kataras/iris/v12"
)

func InitApiRouters(rg iris.Party) {
	// ---- 定时任务与手动触发 ----
	rg.Get("/schedule", schedule)
	rg.Get("/run", runTask)

	// ---- 播放列表与节目单 ----
	rg.Get("/m3u8", generateM3u8)
	rg.Get("/tsM3u8", generateTsM3u8)
	rg.Get("/diyp", generateDiypTxt)
	rg.Get("/epg", generateXmlTv)
	rg.Get("/epgjson", generateEpgJson)

	// ---- 频道管理（API.md 第 12 节）----
	rg.Get("/channel/list", listChannels)
	rg.Get("/channel/m3u8", channelM3u8)
	rg.Post("/channel/toggle", toggleChannel)
	rg.Post("/channel/rename", renameChannel)
	rg.Post("/channel/sort", sortChannels)
	rg.Post("/channel/custom/add", addCustomChannel)
	rg.Post("/channel/custom/update", updateCustomChannel)
	rg.Post("/channel/custom/delete", deleteCustomChannel)

	// ---- EPG 配置（API.md 第 11 节）----
	rg.Get("/epg/config", getEpgConfig)
	rg.Post("/epg/config", updateEpgConfig)

	// ---- 系统 / 运维（API.md 第 1、3、5、6、7、8、14、15 节）----
	rg.Get("/health", health)
	rg.Get("/requests", requests)
	rg.Get("/network-check", networkCheck)
	rg.Get("/version-check", versionCheck)
	rg.Get("/self-upgrade", selfUpgrade)
	rg.Get("/log/stream", logStream)
	rg.Get("/admin/log-level", getLogLevel)
	rg.Post("/admin/log-level", setLogLevel)
	rg.Get("/status.html", statusPage)
}

// runTask GET /api/run?task=xxx[&ref=true]
// 立即返回 OK，实际任务在后台执行（进度可通过 /api/log/stream 观察）
func runTask(ctx iris.Context) {
	taskName := ctx.FormValue("task")
	ref := ctx.FormValue("ref") == "true"

	go func() {
		// 用任务名做 singleflight key：不同任务不会被合并，同一任务并发只跑一次
		global.ConcurrencyControl.Do("run:"+taskName, func() (interface{}, error) {
			switch taskName {
			case "clean-ch":
				auth.CleanChannelData()
			case "clean-chi":
				auth.CleanChannelInfoData()
			case "clean-epg":
				auth.CleanEPGDetailsData()
			case "clean":
				auth.CleanChannelData()
				auth.CleanChannelInfoData()
				auth.CleanEPGDetailsData()
			case "update-chi", "update-epg":
				// 认证客户端可能因参数/专网问题未创建成功，这里必须判空，否则会 panic 掉整个进程
				client := auth.GetGlobalClient()
				if client == nil {
					global.LOG.Error("认证客户端未就绪，无法执行任务: " + taskName)
					return nil, nil
				}
				if taskName == "update-chi" {
					client.FetchChannelList()
				} else {
					client.FetchChannelProg(ref)
				}
			case "upload-m3u":
				auth.GenerateAndUploadM3u()
			case "upload-xmltv":
				auth.GenerateAndUploadXmlTv()
			case "upload-xmltv7":
				auth.GenerateAndUploadXmlTvDays7()
			case "upload-epgjson":
				auth.GenerateAndUploadEpgJson()
			case "upload-epgjson7":
				auth.GenerateAndUploadEpgJsonDays7()
			default:
				global.LOG.Warn("未知任务: " + taskName)
			}
			return nil, nil
		})
	}()
	ctx.WriteString("OK")
}

func schedule(ctx iris.Context) {
	type s struct {
		ID       int
		PreTime  time.Time
		NextTime time.Time
	}
	var out []s
	for _, entry := range global.CRON.Entries() {
		out = append(out, s{
			ID:       int(entry.ID),
			PreTime:  entry.Prev,
			NextTime: entry.Next,
		})
	}
	ctx.JSON(out)
}

// m3uCacheKey 生成 M3U 相关缓存键。
//
// 原实现是把参数值直接拼成字符串（`bufStr += all` / `bufStr += ku9`），
// 于是 `?all=true` 与 `?ku9=true` 会拼出同一个 key，两者互相污染缓存。
// 这里改成带参数名的规范键，并加入缓存版本号（频道管理操作会使其自增）。
func m3uCacheKey(name string, extra ...string) string {
	parts := []string{name, global.M3UCacheVersion()}
	parts = append(parts, extra...)
	return utils.CalcMD5KeyForRequest(parts...)
}

// generateM3u8 生成 m3u8 文件（节目去重）
func generateM3u8(ctx iris.Context) {
	udpxy := ctx.FormValue("udpxy")
	scheme := ctx.FormValue("scheme")
	xteve := ctx.FormValue("xteve")
	all := ctx.FormValue("all")
	ref := ctx.FormValue("ref")
	ku9 := ctx.FormValue("ku9")

	reqMD5Key := m3uCacheKey("generateM3u8",
		"udpxy="+udpxy, "scheme="+scheme, "xteve="+xteve, "all="+all, "ku9="+ku9)

	if ref != "true" && global.CACHE.IsExist(reqMD5Key) {
		ctx.Header("Content-Disposition", "attachment; filename=iptv.m3u")
		ctx.Binary(global.CACHE.Get(reqMD5Key).([]byte))
		return
	}

	resp, _, _ := global.ConcurrencyControl.Do(reqMD5Key, func() (interface{}, error) {
		respBytes := auth.GenerateM3u8(udpxy, scheme, xteve, all, ku9)
		if respBytes == nil {
			// 生成失败（例如配置缺 url-tvg）时不要把空结果写进缓存，
			// 否则接下来 default_timeout 分钟内所有请求都拿到空列表。
			global.LOG.Error("生成 M3U8 失败，本次结果不缓存")
			return nil, nil
		}
		timeOut := time.Duration(global.CONFIG.Cache.DefTimeOut)
		global.CACHE.Put(reqMD5Key, respBytes, time.Minute*timeOut)
		utils.SaveToLogDir(respBytes, "iptv.m3u")
		return respBytes, nil
	})

	ctx.Header("Content-Disposition", "attachment; filename=iptv.m3u")
	if resp == nil {
		ctx.StatusCode(iris.StatusInternalServerError)
		ctx.WriteString("生成失败，请查看日志")
		return
	}
	ctx.Binary(resp.([]byte))
}

// generateTsM3u8 生成直播/时移 m3u8
func generateTsM3u8(ctx iris.Context) {
	ref := ctx.FormValue("ref")
	udpxy := ctx.FormValue("udpxy")
	scheme := ctx.FormValue("scheme")
	xteve := ctx.FormValue("xteve")
	all := ctx.FormValue("all")

	reqMD5Key := m3uCacheKey("generateTsM3u8",
		"udpxy="+udpxy, "scheme="+scheme, "xteve="+xteve, "all="+all)

	if ref != "true" && global.CACHE.IsExist(reqMD5Key) {
		ctx.Header("Content-Disposition", "attachment; filename=iptv-ts.m3u")
		ctx.Binary(global.CACHE.Get(reqMD5Key).([]byte))
		return
	}

	resp, _, _ := global.ConcurrencyControl.Do(reqMD5Key, func() (interface{}, error) {
		respBytes := auth.GenerateTimeShiftM3u8(udpxy, scheme, xteve, all)
		if respBytes == nil {
			global.LOG.Error("生成时移 M3U8 失败，本次结果不缓存")
			return nil, nil
		}
		timeOut := time.Duration(global.CONFIG.Cache.DefTimeOut)
		global.CACHE.Put(reqMD5Key, respBytes, time.Minute*timeOut)
		utils.SaveToLogDir(respBytes, "iptv-ts.m3u")
		return respBytes, nil
	})

	ctx.Header("Content-Disposition", "attachment; filename=iptv-ts.m3u")
	if resp == nil {
		ctx.StatusCode(iris.StatusInternalServerError)
		ctx.WriteString("生成失败，请查看日志")
		return
	}
	ctx.Binary(resp.([]byte))
}

// generateDiypTxt 生成 DIYP 格式频道列表
func generateDiypTxt(ctx iris.Context) {
	ref := ctx.FormValue("ref")
	udpxy := ctx.FormValue("udpxy")
	scheme := ctx.FormValue("scheme")
	xteve := ctx.FormValue("xteve")
	all := ctx.FormValue("all")

	reqMD5Key := m3uCacheKey("generateDiyp",
		"udpxy="+udpxy, "scheme="+scheme, "xteve="+xteve, "all="+all)

	if ref != "true" && global.CACHE.IsExist(reqMD5Key) {
		ctx.Header("Content-Disposition", "attachment; filename=iptvdiyp.txt")
		ctx.Binary(global.CACHE.Get(reqMD5Key).([]byte))
		return
	}

	resp, _, _ := global.ConcurrencyControl.Do(reqMD5Key, func() (interface{}, error) {
		respBytes := auth.GenerateDiyp(udpxy, scheme, xteve, all)
		if respBytes == nil {
			global.LOG.Error("生成 DIYP 失败，本次结果不缓存")
			return nil, nil
		}
		timeOut := time.Duration(global.CONFIG.Cache.DefTimeOut)
		global.CACHE.Put(reqMD5Key, respBytes, time.Minute*timeOut)
		utils.SaveToLogDir(respBytes, "iptvdiyp.txt")
		return respBytes, nil
	})

	ctx.Header("Content-Disposition", "attachment; filename=iptvdiyp.txt")
	if resp == nil {
		ctx.StatusCode(iris.StatusInternalServerError)
		ctx.WriteString("生成失败，请查看日志")
		return
	}
	ctx.Binary(resp.([]byte))
}

// generateEpgJson 生成 JSON 格式节目单
func generateEpgJson(ctx iris.Context) {
	days := ctx.FormValue("days")
	daysAgo, err := strconv.Atoi(days)
	if err != nil {
		daysAgo = 1 // default to 1 day
	}

	ref := ctx.FormValue("ref")
	reqMD5Key := m3uCacheKey("generateEpgJson", "days="+strconv.Itoa(daysAgo))

	if ref != "true" && global.CACHE.IsExist(reqMD5Key) {
		ctx.ContentType("application/json")
		ctx.Binary(global.CACHE.Get(reqMD5Key).([]byte))
		return
	}

	resp, _, _ := global.ConcurrencyControl.Do(reqMD5Key, func() (interface{}, error) {
		epgBytes, err := auth.GenerateEpgJson(daysAgo)
		if err != nil || epgBytes == nil {
			global.LOG.Error("生成 EPG JSON 失败，本次结果不缓存")
			return nil, err
		}
		timeOut := time.Duration(global.CONFIG.Cache.DefTimeOut)
		global.CACHE.Put(reqMD5Key, epgBytes, time.Minute*timeOut)
		utils.SaveToLogDir(epgBytes, "epg.json")
		return epgBytes, nil
	})

	if resp == nil {
		ctx.StatusCode(iris.StatusInternalServerError)
		return
	}
	ctx.ContentType("application/json")
	ctx.Binary(resp.([]byte))
}

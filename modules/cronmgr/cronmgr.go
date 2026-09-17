// Package cronmgr 统一注册与热更新定时任务。
//
// 从 main.go 抽出来是为了支持「在线修改 fetch_cron 后立即生效」：
// 原先定时任务注册散落在 main 里且吞掉了 AddFunc 的错误，
// cron 表达式写错时任务会静默地从未注册。
package cronmgr

import (
	"errors"
	"time"

	"iptv-spider-sh/global"
	"iptv-spider-sh/modules/auth"
	"iptv-spider-sh/utils"

	"github.com/golang-module/carbon"
	"github.com/robfig/cron/v3"
)

// 与 initialize.InitCron 的 cron.WithSeconds() 保持一致：秒级、6 段
var parser = cron.NewParser(
	cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
)

var (
	client   *auth.Client
	epgEntry cron.EntryID
)

// ValidateSpec 校验 cron 表达式（秒级 6 段）
func ValidateSpec(spec string) error {
	if spec == "" {
		return errors.New("Cron 表达式不能为空")
	}
	_, err := parser.Parse(spec)
	return err
}

// RegisterAll 注册全部定时任务（main 启动时调用一次）
func RegisterAll(c *auth.Client) {
	client = c

	addCron("@daily", "获取频道信息列表", func() {
		if client != nil {
			client.FetchChannelList()
		}
	})

	// EPG 抓取任务单独记录 entry id，便于在线替换
	if err := UpdateEPGSchedule(global.CONFIG.Epg.FetchCron); err != nil {
		global.LOG.Error("注册 EPG 定时任务失败: " + err.Error())
	}

	addCron("@every 2h", "清理过期数据", func() {
		auth.CleanEPGDetailsData()
		auth.CleanChannelInfoData()
		auth.CleanChannelData()
	})

	registerUploadCrons()
}

// UpdateEPGSchedule 注册或替换 EPG 抓取定时任务（热更新，无需重启）
func UpdateEPGSchedule(spec string) error {
	if global.CRON == nil {
		return errors.New("CRON 未初始化")
	}
	if err := ValidateSpec(spec); err != nil {
		return err
	}

	if epgEntry != 0 {
		global.CRON.Remove(epgEntry)
		epgEntry = 0
	}

	// 注意：闭包里引用的 entryID 必须先声明，
	// 不能用 `entryID, err := AddFunc(... func(){ ... entryID ... })` ——
	// 短变量声明的作用域从声明语句之后才开始，闭包内会报 undefined。
	var entryID cron.EntryID
	entryID, err := global.CRON.AddFunc(spec, func() {
		// cron 在自己的 goroutine 里跑任务且不 recover：这里兜底，避免一次坏响应终止进程
		utils.SafeRun("cron:获取频道节目单列表", func() {
			if client != nil {
				client.FetchChannelProg(false)
			}
		})
		logNext("Task: 获取频道节目单列表", entryID)
	})
	if err != nil {
		return err
	}
	epgEntry = entryID
	logNext("Add Task: 获取频道节目单列表", entryID)
	return nil
}

// NextEPG 下一次 EPG 抓取时间（零值表示未注册）
func NextEPG() time.Time {
	if global.CRON == nil || epgEntry == 0 {
		return time.Time{}
	}
	return global.CRON.Entry(epgEntry).Next
}

func registerUploadCrons() {
	ossConfig := global.CONFIG.OSS
	scpConfig := global.CONFIG.SCP

	ossEnabled := ossConfig.Enable && ossConfig.UploadCron != ""
	scpEnabled := scpConfig.Enable && scpConfig.UploadCron != ""

	if !ossEnabled && !scpEnabled {
		global.LOG.Info("OSS / SCP 上传均未启用，跳过上传定时任务")
		return
	}

	if ossEnabled {
		addCron(ossConfig.UploadCron, "上传文件至OSS", func() {
			auth.GenerateAndUploadM3u()
			auth.GenerateAndUploadXmlTv()
			auth.GenerateAndUploadDiyp("oss")
		})
		addCron("0 25 0,12 * * *", "上传至OSS-7D", func() {
			auth.GenerateAndUploadXmlTvDays7()
		})
	}

	if scpEnabled {
		// GenerateAndUploadM3u / GenerateAndUploadXmlTv 内部是 OSS 上传，
		// 只启用 SCP 时调用它们只会刷「未配置存储服务」，这里只跑真正支持 SCP 的路径。
		addCron(scpConfig.UploadCron, "上传文件至SCP", func() {
			auth.GenerateAndUploadDiyp("scp")
		})
	}
}

// addCron 注册任务并显式处理表达式错误（不再用 _ 吞掉）
func addCron(spec, task string, fn func()) cron.EntryID {
	if global.CRON == nil {
		global.LOG.Error("CRON 未初始化，无法注册任务: " + task)
		return 0
	}
	// 同上：entryID 必须先声明，闭包内才能引用
	var entryID cron.EntryID
	entryID, err := global.CRON.AddFunc(spec, func() {
		// 兜底 panic，避免单个任务失败导致进程退出
		utils.SafeRun("cron:"+task, fn)
		logNext("Task: "+task, entryID)
	})
	if err != nil {
		global.LOG.Error("注册定时任务失败 [" + task + "] 表达式=" + spec + " : " + err.Error())
		return 0
	}
	logNext("Add Task: "+task, entryID)
	return entryID
}

func logNext(msg string, id cron.EntryID) {
	if global.CRON == nil {
		return
	}
	entry := global.CRON.Entry(id)
	if entry.Next.IsZero() {
		global.LOG.Warn(msg + " NextTime: (任务未注册或已失效)")
		return
	}
	global.LOG.Info(msg + " NextTime: " + carbon.FromStdTime(entry.Next).ToString())
}

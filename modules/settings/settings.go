// Package settings 管理落库的 EPG 配置（epg_configs 单行表）。
//
// 语义（与项目 README 一致）：
//   - 首次启动：把 config.yaml 的 epg 段写入数据库
//   - 之后启动：以数据库为准覆盖运行时配置，config.yaml 不再覆盖它
//   - 在线修改：POST /api/epg/config 写库并立即热生效
package settings

import (
	"errors"

	"iptv-spider-sh/global"
	"iptv-spider-sh/model"
	"iptv-spider-sh/modules/loghub"

	"gorm.io/gorm"
)

// 单行配置表的固定主键
const singleRowID uint = 1

// Load 启动时调用：确保存在配置行并应用到运行时
func Load() {
	if global.DB == nil {
		global.LOG.Warn("数据库未就绪，跳过 EPG 配置加载")
		return
	}

	var row model.EpgConfig
	err := global.DB.First(&row, singleRowID).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		row = fromConfig()
		row.ID = singleRowID
		if e := global.DB.Create(&row).Error; e != nil {
			global.LOG.Error("初始化 epg_configs 失败: " + e.Error())
			return
		}
		global.LOG.Info("已用 config.yaml 的 epg 段初始化 epg_configs 表")
	case err != nil:
		global.LOG.Error("读取 epg_configs 失败: " + err.Error())
		return
	default:
		global.LOG.Info("已从数据库加载 EPG 配置（数据库优先于 config.yaml）")
	}

	Apply(&row)
}

// fromConfig 用当前内存配置（来自 config.yaml）构造配置行
func fromConfig() model.EpgConfig {
	e := global.CONFIG.Epg
	return model.EpgConfig{
		Generator:         e.Generator,
		Source:            e.Source,
		XmlUrl:            e.XmlUrl,
		RtspUrl:           e.RtspUrl,
		RtpUrl:            e.RtpUrl,
		LogoUrl:           e.LogoUrl,
		FetchCron:         e.FetchCron,
		Playseek:          e.Playseek,
		LogLevel:          loghub.LevelString(),
		ApiExternalAccess: true,
	}
}

// Apply 把配置行应用到运行时（generate.go 每次请求都读 global.CONFIG.Epg，所以改这里即热生效）
func Apply(row *model.EpgConfig) {
	if global.CONFIG == nil {
		return
	}
	epg := &global.CONFIG.Epg
	epg.Generator = row.Generator
	epg.Source = row.Source
	epg.XmlUrl = row.XmlUrl
	epg.RtspUrl = row.RtspUrl
	epg.RtpUrl = row.RtpUrl
	epg.LogoUrl = row.LogoUrl
	epg.FetchCron = row.FetchCron
	epg.Playseek = row.Playseek

	if row.LogLevel != "" {
		if err := loghub.SetLevel(row.LogLevel); err != nil {
			global.LOG.Warn("应用日志级别失败: " + err.Error())
		}
	}
}

// Current 返回当前生效配置（数据库优先；无记录时按内存配置合成，保证接口始终可用）
func Current() model.EpgConfig {
	if global.DB != nil {
		var row model.EpgConfig
		if err := global.DB.First(&row, singleRowID).Error; err == nil {
			return row
		}
	}
	row := fromConfig()
	row.ID = singleRowID
	return row
}

// Patch 局部更新请求（指针字段用于区分「未传」与「传空值」）
type Patch struct {
	Generator         *string `json:"generator"`
	Source            *string `json:"source"`
	XmlUrl            *string `json:"xml_url"`
	RtspUrl           *string `json:"rtsp_url"`
	RtpUrl            *string `json:"rtp_url"`
	LogoUrl           *string `json:"logo_url"`
	FetchCron         *string `json:"fetch_cron"`
	Playseek          *string `json:"playseek"`
	LogLevel          *string `json:"log_level"`
	ApiExternalAccess *bool   `json:"api_external_access"`
}

// Save 持久化并按需热生效；返回 fetch_cron 是否发生变化（供调用方热更新定时任务）
func Save(p Patch) (cronChanged bool, err error) {
	if global.DB == nil {
		return false, errors.New("数据库未就绪")
	}

	row := Current()
	row.ID = singleRowID
	if p.Generator != nil {
		row.Generator = *p.Generator
	}
	if p.Source != nil {
		row.Source = *p.Source
	}
	if p.XmlUrl != nil {
		row.XmlUrl = *p.XmlUrl
	}
	if p.RtspUrl != nil {
		row.RtspUrl = *p.RtspUrl
	}
	if p.RtpUrl != nil {
		row.RtpUrl = *p.RtpUrl
	}
	if p.LogoUrl != nil {
		row.LogoUrl = *p.LogoUrl
	}
	if p.Playseek != nil {
		row.Playseek = *p.Playseek
	}
	if p.LogLevel != nil {
		if _, e := loghub.ParseLevel(*p.LogLevel); e != nil {
			return false, e
		}
		row.LogLevel = *p.LogLevel
	}
	if p.ApiExternalAccess != nil {
		row.ApiExternalAccess = *p.ApiExternalAccess
	}
	if p.FetchCron != nil {
		if *p.FetchCron != row.FetchCron {
			cronChanged = true
		}
		row.FetchCron = *p.FetchCron
	}

	// 单行表：先删后插会重置 id，这里直接按主键保存
	if err := global.DB.Save(&row).Error; err != nil {
		return false, err
	}

	Apply(&row)
	return cronChanged, nil
}

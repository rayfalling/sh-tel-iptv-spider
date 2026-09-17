package model

// EpgConfig EPG 配置（单行配置表 epg_configs，固定 id=1）
//
// 首次启动时由 config.yaml 的 epg 段写入，之后以数据库为准（与项目 README 描述一致），
// 可通过 GET/POST /api/epg/config 在线修改。
type EpgConfig struct {
	ID                uint   `gorm:"primarykey" json:"-"`
	Generator         string `gorm:"size:255;default:Deny;comment:XMLTV生成器名称" json:"generator"`
	Source            string `gorm:"size:255;default:Shanghai Telecom Iptv Spider;comment:数据源名称" json:"source"`
	XmlUrl            string `gorm:"size:512;comment:EPG XML 地址（M3U 头 url-tvg）" json:"xml_url"`
	RtspUrl           string `gorm:"size:512;comment:RTSP 代理地址（回放流前缀）" json:"rtsp_url"`
	RtpUrl            string `gorm:"size:512;comment:RTP 代理地址（直播流前缀）" json:"rtp_url"`
	LogoUrl           string `gorm:"size:512;comment:Logo 前缀地址" json:"logo_url"`
	FetchCron         string `gorm:"size:64;comment:节目单定时抓取 Cron（秒级，6 段）" json:"fetch_cron"`
	Playseek          string `gorm:"size:255;comment:回看 seek 参数模板" json:"playseek"`
	LogLevel          string `gorm:"size:16;default:info;comment:日志级别" json:"log_level"`
	ApiExternalAccess bool   `gorm:"default:true;comment:是否允许外部直接做写操作" json:"api_external_access"`
}

// TableName 固定表名，避免 GORM 复数化差异
func (EpgConfig) TableName() string {
	return "epg_configs"
}

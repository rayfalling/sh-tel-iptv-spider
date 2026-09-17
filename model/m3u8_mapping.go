package model

import "gorm.io/gorm"

// M3u8Mapping m3u 部分字段映射
//
// 除了作者原有的 logo / 分组映射，这里补齐了频道管理 API 需要的字段
// （custom_name / sort_order / is_custom / tvg_id / igmp），
// 使「频道显隐、重命名、排序、自定义频道增删改」可以只改这一张表。
type M3u8Mapping struct {
	gorm.Model
	CommName     string `gorm:"uniqueIndex;comment:节目通用名称" json:"comm_name"`
	Logo         string `gorm:"comment:Logo 地址" json:"logo"`
	AutoGroups   string `gorm:"comment:自动分组" json:"auto_groups"`
	CustomGroups string `gorm:"comment:自定义组分类" json:"custom_groups"`
	CustomName   string `gorm:"comment:自定义显示名称（为空则用 CommName）" json:"custom_name"`
	SortOrder    int    `gorm:"default:0;comment:排序权重（越小越靠前，0=默认排序）" json:"sort_order"`
	IsCustom     bool   `gorm:"default:false;index;comment:是否为自定义频道" json:"is_custom"`
	TvgId        string `gorm:"comment:tvg-id 标识（EPG 匹配用）" json:"tvg_id"`
	Igmp         string `gorm:"comment:IGMP 组播地址（自定义频道用）" json:"igmp"`
}

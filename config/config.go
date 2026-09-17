package config

type Server struct {
	Zap    Zap    `mapstructure:"zap" json:"zap" yaml:"zap"`
	Redis  Redis  `mapstructure:"redis" json:"redis" yaml:"redis"`
	System System `mapstructure:"system" json:"system" yaml:"system"`
	Cache  Cache  `mapstructure:"cache" json:"cache" yaml:"cache"`
	Mysql  Mysql  `mapstructure:"mysql" json:"mysql" yaml:"mysql"`
	Sqlite Sqlite `mapstructure:"sqlite" json:"sqlite" yaml:"sqlite"`
	// 原先这两个 tag 笔误为 apstructure（少个 m），只靠 mapstructure 的字段名兜底才生效，此处修正
	Stb       Stb       `mapstructure:"stb" json:"stb" yaml:"stb"`
	Epg       Epg       `mapstructure:"epg" json:"epg" yaml:"epg"`
	OSS       OSS       `mapstructure:"oss" json:"oss" yaml:"oss"`
	SCP       OSS       `mapstructure:"scp" json:"scp" yaml:"scp"`
	AccessLog AccessLog `mapstructure:"access-log" json:"accessLog" yaml:"access-log"`
}

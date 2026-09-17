package config

type AccessLog struct {
	Enabled      bool   `mapstructure:"enabled" json:"enabled" yaml:"enabled"`                    // 是否启用访问日志
	Director     string `mapstructure:"director" json:"director" yaml:"director"`                 // 日志文件夹
	Format       string `mapstructure:"format" json:"format" yaml:"format"`                       // 输出格式：json 或 console
	LogInConsole bool   `mapstructure:"log-in-console" json:"logInConsole" yaml:"log-in-console"` // 输出到控制台
	MaxAge       int    `mapstructure:"max-age" json:"maxAge" yaml:"max-age"`                     // 日志保留天数
	RotationTime int    `mapstructure:"rotation-time" json:"rotationTime" yaml:"rotation-time"`   // 轮转时间（小时）
}

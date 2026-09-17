package initialize

import (
	"iptv-spider-sh/global"
	"iptv-spider-sh/initialize/internal"
	"iptv-spider-sh/model"
	"os"

	"github.com/glebarez/sqlite"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Gorm() *gorm.DB {
	switch global.CONFIG.System.DbType {
	case "mysql":
		return gormMysql()
	case "sqlite":
		return gormSqlite()
	}
	panic("数据库连接出错！")
}

func MysqlTables(db *gorm.DB) {
	err := db.AutoMigrate(
		model.Channel{},
		model.ChannelInfo{},
		model.AuthInfo{},
		model.EPGDetails{},
		model.M3u8Mapping{},
		// EPG 配置单行表：供 /api/epg/config 在线读写（README/API.md 中承诺的表）
		model.EpgConfig{},
	)

	if err != nil {
		global.LOG.Error("register table failed", zap.Any("err", err))
		os.Exit(0)
	}
	global.LOG.Info("register table success")
}

func gormMysql() *gorm.DB {
	m := global.CONFIG.Mysql
	if m.Dbname == "" {
		global.LOG.Error("mysql 未配置 dbname，数据库不可用")
		return nil
	}
	mysqlConfig := mysql.Config{
		DSN:                       m.Dsn(), // DSN data source name
		DefaultStringSize:         191,     // string 类型字段的默认长度
		DisableDatetimePrecision:  true,    // 禁用 datetime 精度，MySQL 5.6 之前的数据库不支持
		DontSupportRenameIndex:    true,    // 重命名索引时采用删除并新建的方式，MySQL 5.7 之前的数据库和 MariaDB 不支持重命名索引
		DontSupportRenameColumn:   true,    // 用 `change` 重命名列，MySQL 8 之前的数据库和 MariaDB 不支持重命名列
		SkipInitializeWithVersion: false,   // 根据版本自动配置
	}
	db, err := gorm.Open(mysql.New(mysqlConfig), gormConfig("mysql"))
	if err != nil {
		global.LOG.Error("连接 mysql 失败: " + err.Error())
		return nil
	}
	sqlDB, err := db.DB()
	if err != nil || sqlDB == nil {
		// 原实现用 _ 忽略错误后直接 sqlDB.SetMaxIdleConns，err 非空时是 nil 解引用 panic
		global.LOG.Error("获取 mysql 连接池失败")
		return nil
	}
	sqlDB.SetMaxIdleConns(m.MaxIdleConns)
	sqlDB.SetMaxOpenConns(m.MaxOpenConns)
	return db
}

func gormSqlite() *gorm.DB {
	s := global.CONFIG.Sqlite
	if s.Path == "" {
		global.LOG.Error("sqlite 未配置 path，数据库不可用")
		return nil
	}
	db, err := gorm.Open(sqlite.Open(s.Path), gormConfig("sqlite"))
	if err != nil {
		global.LOG.Error("打开 sqlite 失败: " + err.Error())
		return nil
	}
	sqlDB, err := db.DB()
	if err != nil || sqlDB == nil {
		global.LOG.Error("获取 sqlite 连接池失败")
		return nil
	}
	sqlDB.SetMaxIdleConns(s.MaxIdleConns)
	sqlDB.SetMaxOpenConns(s.MaxOpenConns)
	return db
}

func gormConfig(dbType string) *gorm.Config {
	config := &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true}
	var logMode string
	switch dbType {
	case "mysql":
		logMode = global.CONFIG.Mysql.LogMode
	case "sqlite":
		logMode = global.CONFIG.Sqlite.LogMode
	default:
		logMode = "info"
	}

	switch logMode {
	case "silent", "Silent":
		config.Logger = internal.Default.LogMode(logger.Silent)
	case "error", "Error":
		config.Logger = internal.Default.LogMode(logger.Error)
	case "warn", "Warn":
		config.Logger = internal.Default.LogMode(logger.Warn)
	case "info", "Info":
		config.Logger = internal.Default.LogMode(logger.Info)
	default:
		config.Logger = internal.Default.LogMode(logger.Info)
	}
	return config
}

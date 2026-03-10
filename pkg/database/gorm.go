package database

import (
	"fmt"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"
)

func Init(cfg Config, opts ...gorm.Option) *gorm.DB {

	db, err := gorm.Open(open(cfg.Master), opts...)
	if err != nil {
		panic(err)
	}

	// 集群
	if len(cfg.Slavers) != 0 {
		slaverOpens := make([]gorm.Dialector, 0, len(cfg.Slavers))
		for _, slaver := range cfg.Slavers {
			slaverOpens = append(slaverOpens, open(slaver))
		}
		err = db.Use(dbresolver.Register(dbresolver.Config{
			Sources:  []gorm.Dialector{open(cfg.Master)},
			Replicas: slaverOpens,
			Policy:   dbresolver.RandomPolicy{}, // 从库之间随机选择
		}))
		if err != nil {
			panic(err)
		}
	}

	return db
}

func open(cfg DBConfig) (d gorm.Dialector) {
	switch cfg.Driver {
	case MYSQL:
		dsn := getMysqlDSN(cfg)
		d = mysql.Open(dsn)
	case POSTGRES:
		dsn := getPostgresDSN(cfg)
		d = postgres.Open(dsn)
	default:
		panic("unknown db driver")
	}
	return
}

func getMysqlDSN(cfg DBConfig) string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.Username,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.DbName)
}

func getPostgresDSN(cfg DBConfig) string {
	return fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=disable TimeZone=Asia/Shanghai",
		cfg.Host,
		cfg.Username,
		cfg.Password,
		cfg.DbName,
		cfg.Port)
}

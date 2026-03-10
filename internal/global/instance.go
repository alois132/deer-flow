package global

import (
	"context"
	"github.com/alois132/deer-flow/pkg/database"
	"github.com/alois132/deer-flow/pkg/log"
	redis2 "github.com/alois132/deer-flow/pkg/redis"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var (
	db     *gorm.DB
	cache  *redis.Client
	ctx    context.Context
	logger *zap.Logger
)

func InitInstances(cfg *Config) {
	ctx = context.Background()
	db = database.Init(cfg.Database)
	cache = redis2.MustNew(cfg.Redis)
	log.InitLog(cfg.Log)
}

func GetDB() *gorm.DB {
	return db
}

func GetCache() *redis.Client {
	return cache
}

func GetCtx() context.Context {
	return ctx
}

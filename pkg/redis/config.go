package redis

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
)

type Config struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// 获取redis客户端
func New(config Config) (*redis.Client, error) {

	opt := &redis.Options{
		Addr:     getAdder(config),
		Password: config.Password,
		DB:       config.DB,
	}

	client := redis.NewClient(opt)

	_, err := client.Ping(context.Background()).Result()
	if err != nil {
		return nil, err
	}

	return client, nil
}

func MustNew(config Config) *redis.Client {
	cilent, err := New(config)
	if err != nil {
		panic(err)
	}

	return cilent
}

func getAdder(config Config) string {
	return fmt.Sprintf("%s:%d", config.Host, config.Port)
}

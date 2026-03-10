package global

import (
	"github.com/alois132/deer-flow/pkg/database"
	"github.com/alois132/deer-flow/pkg/llm"
	"github.com/alois132/deer-flow/pkg/log"
	"github.com/alois132/deer-flow/pkg/redis"
	"github.com/alois132/deer-flow/pkg/sandbox"
	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig    `mapstructure:"server"`
	Database database.Config `mapstructure:"database"`
	Redis    redis.Config    `mapstructure:"redis"`
	Agent    AgentConfig     `mapstructure:"agent"`
	Sandbox  sandbox.Config  `mapstructure:"sandbox"`
	Log      log.Config      `mapstructure:"log"`
}

type ServerConfig struct {
	Port         int    `mapstructure:"port"`
	Mode         string `mapstructure:"mode"`
	ReadTimeout  int    `mapstructure:"read_timeout"`
	WriteTimeout int    `mapstructure:"write_timeout"`
}

type AgentConfig struct {
	DefaultLLM *llm.Config `mapstructure:"default_llm"`
	MemoryLLM  *llm.Config `mapstructure:"memory_llm"`
}

var config *Config

func Load(path string) *Config {
	viper.SetConfigFile(path)
	viper.AutomaticEnv()
	if err := viper.ReadInConfig(); err != nil {
		panic(err)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		panic(err)
	}

	config = &cfg
	InitInstances(config)
	return config
}

func GetCfg() *Config {
	return config
}

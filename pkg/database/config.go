package database

// 自定义类型
// 选项模式，不允许外界实现
type driver string

// 常量
const (
	POSTGRES driver = "postgres"
	MYSQL    driver = "mysql"
)

type DBConfig struct {
	Driver   driver `json:"driver" mapstructure:"driver"`
	Host     string `json:"host" mapstructure:"host"`
	Port     int    `json:"port" mapstructure:"port"`
	Username string `json:"username" mapstructure:"username"`
	Password string `json:"password" mapstructure:"password"`
	DbName   string `json:"db_name" mapstructure:"db_name"`
}

type Config struct {
	Master  DBConfig   `mapstructure:"master"`
	Slavers []DBConfig `mapstructure:"slavers"`
}

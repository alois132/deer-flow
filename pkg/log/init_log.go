package log

import (
	"github.com/alois132/deer-flow/pkg/log/zlog"
)

func InitLog(config Config) {
	logger := GetZap(config)
	zlog.InitLogger(logger)
}

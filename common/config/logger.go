package config

import "github.com/ykhdr/crack-hash/common/logging"

type LogConfig struct {
	LogLevel string `kdl:"log-level"`
}

func (c *LogConfig) GetLogLevel() string {
	return c.LogLevel
}

type hasLogLevel interface {
	GetLogLevel() string
}

func setupLogger(cfg any) {
	var logLevel logging.Level
	logCfg, ok := cfg.(hasLogLevel)
	if !ok {
		logLevel = logging.InfoLevel
	} else {
		logLevel = logging.ParseLevel(logCfg.GetLogLevel())
	}
	logging.Setup(logLevel)
}

package config

import (
	"fmt"
	"github.com/ykhdr/crack-hash/common/internal/kdl"
)

const defaultConfigPath = "./config/config.kdl"

func InitializeConfig[T any](args []string, defaultCfg T) (*T, error) {
	var configPath string
	if len(args) == 0 {
		configPath = defaultConfigPath
	} else {
		configPath = args[0]
	}
	var nilT *T
	config, err := kdl.Unmarshal[T](configPath, defaultCfg)
	if err != nil {
		return nilT, fmt.Errorf("unmarshal kdl: %w", err)
	}
	setupLogger(config)
	return &config, nil
}

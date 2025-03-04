package kdl

import (
	"github.com/sblinch/kdl-go"
	"os"
)

func Unmarshal[T any](kdlPath string, defaultCfg T) (T, error) {
	var nilT T
	data, err := os.ReadFile(kdlPath)
	if err != nil {
		return nilT, err
	}
	if err := kdl.Unmarshal(data, &defaultCfg); err != nil {
		return nilT, err
	}
	return defaultCfg, nil
}

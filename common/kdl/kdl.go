package kdl

import (
	"github.com/sblinch/kdl-go"
	"os"
)

func Unmarshal[T any](kdlPath string) (*T, error) {
	data, err := os.ReadFile(kdlPath)
	if err != nil {
		return nil, err
	}

	var obj T
	if err := kdl.Unmarshal(data, &obj); err != nil {
		return nil, err
	}
	return &obj, nil
}

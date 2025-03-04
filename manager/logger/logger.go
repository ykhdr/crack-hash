package log

import (
	log "log/slog"
	"os"
)

func init() {
	logger := log.New(log.NewTextHandler(os.Stdout, nil)).WithGroup("MANAGER")
	log.SetDefault(logger)
}

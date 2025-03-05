package log

import (
	log "log/slog"
	"os"
)

func init() {
	logger := log.New(log.NewTextHandler(os.Stdout, &log.HandlerOptions{
		Level: log.LevelDebug,
	}))
	log.SetDefault(logger)
}

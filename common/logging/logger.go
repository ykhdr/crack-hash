package logging

import (
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"io"
	"os"
	"strings"
	"time"
)

type Level zerolog.Level

const InfoLevel = Level(zerolog.InfoLevel)

func (l Level) toZerolog() zerolog.Level {
	return zerolog.Level(l)
}

func Setup(level Level) {
	zerolog.SetGlobalLevel(level.toZerolog())
	var writer io.Writer
	switch level.toZerolog() {
	case zerolog.DebugLevel:
		writer = zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) {
			w.TimeFormat = time.RFC3339
		})
	default:
		writer = os.Stdout
	}
	log.Logger = zerolog.
		New(writer).
		With().
		Timestamp().
		Caller().
		Logger()
}

func ParseLevel(lvl string) Level {
	parsedLevel, err := zerolog.ParseLevel(strings.ToLower(lvl))
	if err != nil || parsedLevel == zerolog.NoLevel {
		return Level(zerolog.InfoLevel)
	}
	return Level(parsedLevel)
}

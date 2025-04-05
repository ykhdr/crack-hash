package strategy

import (
	"github.com/rs/zerolog"
	"github.com/ykhdr/crack-hash/manager/pkg/messages"
	"sync"
)

type emptyStrategy struct {
	l zerolog.Logger
}

var once sync.Once
var emptyStrategyInstance *emptyStrategy

func newEmptyStrategy(l zerolog.Logger) *emptyStrategy {
	once.Do(func() {
		emptyStrategyInstance = &emptyStrategy{
			l: l.With().
				Str("domain", "hashcrack").
				Str("type", "strategy").
				Str("strategy", "empty").
				Logger(),
		}
	})
	return emptyStrategyInstance
}

func (s *emptyStrategy) CrackMd5(req *messages.CrackHashManagerRequest) CrackResult {
	s.l.Debug().
		Str("req-id", req.RequestId).
		Int("part-number", req.PartNumber).
		Str("hash", req.Hash).
		Int("max-length", req.MaxLength).
		Msg("cracking hash")

	return &crackResult{
		found: []string{},
	}
}

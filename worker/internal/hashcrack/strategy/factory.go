package strategy

import log "github.com/rs/zerolog/log"

type Type int

const (
	EmptyStrategyType Type = iota
	BruteForceStrategyType
)

const (
	bruteForceStrategyName = "brute-force"
)

func NewStrategy(strategyType Type) Strategy {
	switch strategyType {
	case BruteForceStrategyType:
		return newBruteForceStrategy(log.Logger)
	default:
		return newEmptyStrategy(log.Logger)
	}
}

func ParseStrategyName(name string) Type {
	switch name {
	case bruteForceStrategyName:
		return BruteForceStrategyType
	default:
		return EmptyStrategyType
	}
}

func DefaultStrategyStr() string {
	return bruteForceStrategyName
}

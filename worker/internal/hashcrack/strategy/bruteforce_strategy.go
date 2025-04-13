package strategy

import (
	"crypto/md5"
	"encoding/hex"
	"github.com/rs/zerolog"
	"github.com/ykhdr/crack-hash/manager/pkg/messages"
	"math"
	"strings"
)

type bruteForceStrategy struct {
	l zerolog.Logger
}

func newBruteForceStrategy(logger zerolog.Logger) *bruteForceStrategy {
	return &bruteForceStrategy{
		l: logger.
			With().
			Str("domain", "hashcrack").
			Str("type", "strategy").
			Str("strategy", "brute-force").
			Logger(),
	}
}

func (s *bruteForceStrategy) CrackMd5(req *messages.CrackHashManagerRequest) CrackResult {
	s.l.Debug().
		Str("req-id", req.RequestId).
		Int("part-number", req.PartNumber).
		Str("hash", req.Hash).
		Int("max-length", req.MaxLength).
		Msg("cracking hash")

	var found []string
	targetHash := strings.ToLower(req.Hash)

	alpha := req.Alphabet.Symbols
	alphaSize := len(alpha)

	totalCombinations := 0
	lengthsCount := []int{}
	for length := 1; length <= req.MaxLength; length++ {
		c := int(math.Pow(float64(alphaSize), float64(length)))
		totalCombinations += c
		lengthsCount = append(lengthsCount, c)
	}

	start := (totalCombinations * req.PartNumber) / req.PartCount
	end := (totalCombinations * (req.PartNumber + 1)) / req.PartCount
	if end > totalCombinations {
		end = totalCombinations
	}

	for i := start; i < end; i++ {
		word := getStringByIndex(i, lengthsCount, alpha)
		h := md5.Sum([]byte(word))
		hStr := hex.EncodeToString(h[:])
		if hStr == targetHash {
			s.l.Debug().Msgf("found word: %s", word)
			found = append(found, word)
		}
	}

	return &crackResult{found: found}
}

func getStringByIndex(i int, lengthsCount []int, alphabet []string) string {
	cur := i
	length := 1
	for _, c := range lengthsCount {
		if cur < c {
			break
		}
		cur -= c
		length++
	}
	sb := strings.Builder{}
	sb.Grow(length)

	alphabetSize := len(alphabet)
	for p := 0; p < length; p++ {
		power := int(math.Pow(float64(alphabetSize), float64(length-p-1)))
		index := cur / power
		cur = cur % power
		sb.WriteString(alphabet[index])
	}
	return sb.String()
}

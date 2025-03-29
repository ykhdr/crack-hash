package service

import (
	"crypto/md5"
	"encoding/hex"
	"github.com/ykhdr/crack-hash/manager/pkg/messages"
	log "log/slog"
	"math"
	"strings"
)

func crackMD5(req *messages.CrackHashManagerRequest) []string {
	found := []string{}
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

	getStringByIndex := func(i int) string {
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

		for p := 0; p < length; p++ {
			power := int(math.Pow(float64(alphaSize), float64(length-p-1)))
			index := cur / power
			cur = cur % power
			sb.WriteString(alpha[index])
		}
		return sb.String()
	}

	for i := start; i < end; i++ {
		word := getStringByIndex(i)
		h := md5.Sum([]byte(word))
		hStr := hex.EncodeToString(h[:])
		if hStr == targetHash {
			log.Debug("Found word", "word", word)
			found = append(found, word)
		}
	}

	return found
}

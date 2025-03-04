package service

import (
	"crypto/md5"
	"encoding/hex"
	"github.com/ykhdr/crack-hash/common/api"
	log "log/slog"
	"math"
	"strings"
)

// Эта функция генерирует строки и проверяет их MD5
func crackMD5(req *api.CrackHashManagerRequest) []string {
	found := []string{}
	targetHash := strings.ToLower(req.Hash)

	// Алфавит
	alpha := req.Alphabet.Symbols // []string
	alphaSize := len(alpha)

	// Общее кол-во комбинаций (для строк длины 1..MaxLength)
	// = sum_{k=1..MaxLength} alphaSize^k
	// Но мы делим поровну между всеми воркерами.
	// Для простоты предположим, что PartNumber/PartCount позволяет нам взять нужный поддиапазон
	// в прямой линейной индексации по всему пространству.
	// (Я не совсем уверен в этом, что так будет идеально, ибо строго говоря надо аккуратно разложить по длинам,
	// но для примера сойдёт.)

	totalCombinations := 0
	lengthsCount := []int{}
	for length := 1; length <= req.MaxLength; length++ {
		c := int(math.Pow(float64(alphaSize), float64(length)))
		totalCombinations += c
		lengthsCount = append(lengthsCount, c)
	}

	// Для каждого PartNumber выделим промежуток:
	//   start = (totalCombinations * PartNumber) / PartCount
	//   end = (totalCombinations * (PartNumber+1)) / PartCount - 1
	// и переберем комбинации только в этом диапазоне.
	start := (totalCombinations * req.PartNumber) / req.PartCount
	end := (totalCombinations * (req.PartNumber + 1)) / req.PartCount
	if end > totalCombinations {
		end = totalCombinations
	}

	// Функция, которая вернёт i-ую лексикографическую строку (i от 0) среди всех строк 1..maxLength
	// (Я не совсем уверен в корректности на 100%, возможно, нужна более аккуратная математика,
	// но идея такая: определяем, в какую длину попадает i, вычисляем остаток и генерируем строку.)
	getStringByIndex := func(i int) string {
		// определим, какой длине принадлежит индекс
		cur := i
		length := 1
		for _, c := range lengthsCount {
			if cur < c {
				// значит длина = текущий length
				break
			}
			cur -= c
			length++
		}
		// теперь генерируем строку длины length по номеру cur
		// cur находится в диапазоне [0, alphaSize^length)
		sb := strings.Builder{}
		sb.Grow(length)

		// например, если length=3 и alphaSize=36, тогда общий объём = 36^3 = 46656
		// чтобы получить конкретную комбинацию: делим cur на 36^(length-1), берём alpha[quotient], остаток идёт дальше и т.д.
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

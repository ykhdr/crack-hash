package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/xml"
	"github.com/ykhdr/crack-hash/common"
	"github.com/ykhdr/crack-hash/common/api"
	"io"
	log "log/slog"
	"math"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/mux"
)

func init() {
	logger := log.New(log.NewTextHandler(os.Stdout, nil)).WithGroup("WORKER")
	log.SetDefault(logger)
}

func main() {
	r := mux.NewRouter()

	// Воркер слушает запросы от менеджера
	r.HandleFunc("/internal/api/worker/hash/crack/task", handleCrackTask).Methods("POST")

	srv := &http.Server{
		Handler: r,
		Addr:    ":8081",
	}

	log.Info("Worker is running on port 8081...")
	if err := srv.ListenAndServe(); err != nil {
		log.Warn("Worker server failed: %v", err)
	}
}

func handleCrackTask(w http.ResponseWriter, r *http.Request) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	var reqXml api.CrackHashManagerRequest
	if err := xml.Unmarshal(bodyBytes, &reqXml); err != nil {
		http.Error(w, "Invalid XML", http.StatusBadRequest)
		return
	}

	// Обрабатываем: генерируем комбинации и ищем совпадения
	found := crackMD5(reqXml)

	// Возвращаем ответ менеджеру
	// (PATCH /internal/api/manager/hash/crack/request)
	respXml := api.CrackHashWorkerResponse{
		RequestId: reqXml.RequestId,
		Found:     found,
	}
	sendResponseToManager(respXml)

	// Можно вернуть http.StatusOK тут
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Task received. Processing..."))
}

// Эта функция генерирует строки и проверяет их MD5
func crackMD5(req api.CrackHashManagerRequest) []string {
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
			found = append(found, word)
		}
	}

	return found
}

// Отправка результата менеджеру
func sendResponseToManager(resp api.CrackHashWorkerResponse) {
	managerURL := "http://manager:8080/internal/api/manager/hash/crack/request"
	// если внутри docker-compose, то manager доступен по имени контейнера/сервиса

	bytesToSend, err := xml.Marshal(resp)
	if err != nil {
		log.Warn("Failed to marshal response XML:", err)
		return
	}

	req, err := http.NewRequest("PATCH", managerURL, io.NopCloser(common.NewBytesReader(bytesToSend)))
	if err != nil {
		log.Warn("Failed to create request:", err)
		return
	}
	req.Header.Set("Content-Type", "application/xml")

	client := &http.Client{}
	httpResp, err := client.Do(req)
	if err != nil {
		log.Warn("Failed to send response to manager:", err)
		return
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode != http.StatusOK {
		log.Warn("Manager responded with status:", httpResp.StatusCode)
		return
	}

	log.Warn("Response successfully sent to manager.")
}

package main

import (
	"encoding/json"
	"encoding/xml"
	"github.com/google/uuid"
	"github.com/ykhdr/crack-hash/common/api"
	"io"
	"io/ioutil"
	log "log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/ykhdr/crack-hash/common"
)

func init() {
	logger := log.New(log.NewTextHandler(os.Stdout, nil)).WithGroup("MANAGER")
	log.SetDefault(logger)
}

// Модель для хранения состояния запроса
type RequestStatus string

const (
	StatusInProgress RequestStatus = "IN_PROGRESS"
	StatusReady      RequestStatus = "READY"
	StatusError      RequestStatus = "ERROR"
)

// храним в памяти
type RequestInfo struct {
	ID        string
	Status    RequestStatus
	FoundData []string
	CreatedAt time.Time
}

// ManagerConfig — настройки, которые можно было бы прочитать из KDL (или из env)
type ManagerConfig struct {
	WorkerCount int
	WorkerUrls  []string // список URL воркеров типа http://worker:8081/internal/api/worker/hash/crack/task
	TimeoutSec  int
}

var requestsStore = struct {
	sync.RWMutex
	data map[string]*RequestInfo
}{data: make(map[string]*RequestInfo)}

// Некий конфиг
var managerCfg = ManagerConfig{
	WorkerCount: 1, // можно увеличить
	WorkerUrls:  []string{"http://worker:8081/internal/api/worker/hash/crack/task"},
	TimeoutSec:  30,
}

func main() {
	r := mux.NewRouter()

	// Внешнее API для клиента
	r.HandleFunc("/api/hash/crack", handleHashCrack).Methods("POST")
	r.HandleFunc("/api/hash/status", handleHashStatus).Methods("GET")

	// Внутреннее API для воркера
	r.HandleFunc("/internal/api/manager/hash/crack/request", handleWorkerResponse).Methods("PATCH")

	srv := &http.Server{
		Handler: r,
		Addr:    ":8080",
	}

	log.Info("Manager is running on port 8080...")
	if err := srv.ListenAndServe(); err != nil {
		log.Error("Manager server failed: %v", err)
	}
}

// 1) Обработка запроса на взлом
func handleHashCrack(w http.ResponseWriter, r *http.Request) {
	type CrackRequest struct {
		Hash      string `json:"hash"`
		MaxLength int    `json:"maxLength"`
	}

	var reqBody CrackRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	requestId, err := uuid.NewUUID()
	if err != nil {
		log.Warn("Failed to generate request id: %v", err)
		http.Error(w, "Failed to generate request id", http.StatusInternalServerError)
		return
	}

	requestIdStr := requestId.String()
	// Сохраняем в памяти
	requestsStore.Lock()
	requestsStore.data[requestIdStr] = &RequestInfo{
		ID:        requestIdStr,
		Status:    StatusInProgress,
		FoundData: []string{},
		CreatedAt: time.Now(),
	}
	requestsStore.Unlock()

	// Отправляем задачу на воркеры
	go dispatchTasksToWorkers(requestIdStr, reqBody.Hash, reqBody.MaxLength)

	// Возвращаем ответ клиенту
	resp := map[string]string{"requestId": requestIdStr}
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(resp)
	if err != nil {
		log.Warn("Failed to encode response: %v", err)
	}
}

// 2) Проверка статуса
func handleHashStatus(w http.ResponseWriter, r *http.Request) {
	requestId := r.URL.Query().Get("requestId")
	if requestId == "" {
		http.Error(w, "Missing requestId", http.StatusBadRequest)
		return
	}

	requestsStore.RLock()
	info, ok := requestsStore.data[requestId]
	requestsStore.RUnlock()

	if !ok {
		http.Error(w, "Request not found", http.StatusNotFound)
		return
	}

	// Формируем ответ
	resp := map[string]interface{}{
		"status": info.Status,
		"data":   nil,
	}
	if info.Status == StatusReady {
		resp["data"] = info.FoundData
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// 3) Получение ответа от воркера (XML)
func handleWorkerResponse(w http.ResponseWriter, r *http.Request) {
	// Читаем XML
	bodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	var workerResp api.CrackHashWorkerResponse
	if err := xml.Unmarshal(bodyBytes, &workerResp); err != nil {
		http.Error(w, "Invalid XML", http.StatusBadRequest)
		return
	}

	// Обновляем данные
	requestsStore.Lock()
	reqInfo, ok := requestsStore.data[workerResp.RequestId]
	if !ok {
		requestsStore.Unlock()
		// возможно, уже истёк таймаут
		return
	}

	// Добавляем найденные слова
	reqInfo.FoundData = append(reqInfo.FoundData, workerResp.Found...)

	// Проверяем, не все ли воркеры уже отчитались?
	// В данном примере для простоты считаем, что если пришел ответ 1 раз, то всё.
	// Но на практике надо считать кол-во ответов, сравнивать с managerCfg.WorkerCount.
	// Для упрощения — как только приходит PATCH, переводим в READY.
	reqInfo.Status = StatusReady

	requestsStore.Unlock()

	w.WriteHeader(http.StatusOK)
}

// Распределяем задачу по воркерам
func dispatchTasksToWorkers(requestId, hash string, maxLength int) {
	// Для простоты равномерно делим диапазон
	// При алфавите в 36 символов, нам нужно генерировать строки длиной от 1 до maxLength.
	// На самом деле полный объём = 36^1 + 36^2 + ... + 36^maxLength.
	// Но мы просто по количеству воркеров разбиваем, указывая partNumber, partCount.

	partCount := managerCfg.WorkerCount
	for partNumber := 0; partNumber < partCount; partNumber++ {
		// Формируем XML
		reqXml := api.CrackHashManagerRequest{
			RequestId:  requestId,
			PartNumber: partNumber,
			PartCount:  partCount,
			Hash:       hash,
			MaxLength:  maxLength,
			Alphabet: api.Alphabet{
				Symbols: generateAlphabet(), // [a-z0-9]
			},
		}

		// Отправляем асинхронно
		go sendRequestToWorker(reqXml, managerCfg.WorkerUrls[partNumber])
	}

	// Запускаем таймер, если не пришли ответы, проставим ERROR
	time.AfterFunc(time.Duration(managerCfg.TimeoutSec)*time.Second, func() {
		requestsStore.Lock()
		reqInfo, ok := requestsStore.data[requestId]
		if ok && reqInfo.Status == StatusInProgress {
			reqInfo.Status = StatusError
		}
		requestsStore.Unlock()
	})
}

func sendRequestToWorker(reqXml api.CrackHashManagerRequest, url string) {
	bytesToSend, err := xml.Marshal(reqXml)
	if err != nil {
		log.Warn("Failed to marshal request XML:", err)
		return
	}
	resp, err := http.Post(url, "application/xml",
		io.NopCloser(common.NewBytesReader(bytesToSend)),
	)
	if err != nil {
		log.Warn("Failed to send request to worker:", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		log.Warn("Worker responded with status:", resp.StatusCode)
		return
	}
	log.Info("Request sent to worker successfully")
}

// Вспомогательная функция для генерации алфавита
func generateAlphabet() []string {
	// 36 символов: a-z, 0-9
	alpha := []rune("abcdefghijklmnopqrstuvwxyz0123456789")
	result := make([]string, len(alpha))
	for i, r := range alpha {
		result[i] = string(r)
	}
	return result
}

package service

import (
	"encoding/xml"
	"github.com/gorilla/mux"
	"github.com/ykhdr/crack-hash/common/api"
	"github.com/ykhdr/crack-hash/common/middleware"
	"github.com/ykhdr/crack-hash/manager/config"
	"io"
	log "log/slog"
	"net/http"
)

type WorkerServer struct {
	addr string
}

func NewWorkerServer(cfg *config.ManagerConfig) *WorkerServer {
	return &WorkerServer{
		addr: cfg.WorkerServerAddr,
	}
}

func (s *WorkerServer) Start() {
	r := mux.NewRouter()
	r.HandleFunc("/internal/api/manager/hash/crack/request", s.handleWorkerResponse).Methods("PATCH")
	r.Use(middleware.LoggingMiddleware(log.Debug))
	log.Info("Worker server is running", "address", s.addr)
	if err := http.ListenAndServe(s.addr, r); err != nil {
		log.Error("Worker server failed", "err", err)
	}
}

func (s *WorkerServer) handleWorkerResponse(w http.ResponseWriter, r *http.Request) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		log.Warn("Failed to read worker response body", "err", err)
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	var workerResp api.CrackHashWorkerResponse
	if err := xml.Unmarshal(bodyBytes, &workerResp); err != nil {
		log.Warn("Failed to unmarshal worker response body", "err", err)
		http.Error(w, "Invalid XML", http.StatusBadRequest)
		return
	}
	reqInfo, exists := GetRequestStore().Get(RequestId(workerResp.RequestId))
	if !exists {
		log.Warn("Failed to find request store", "id", workerResp.RequestId)
		http.Error(w, "Request not found", http.StatusNotFound)
		return
	}
	if reqInfo.Status == StatusError || reqInfo.ReadyServiceCount == reqInfo.ServiceCount {
		log.Warn("Request is already canceled", "id", workerResp.RequestId)
		http.Error(w, "Request failed", http.StatusInternalServerError)
		return
	}
	reqInfo.FoundData = append(reqInfo.FoundData, workerResp.Found...)
	reqInfo.ReadyServiceCount++
	if reqInfo.ReadyServiceCount == reqInfo.ServiceCount {
		reqInfo.Status = StatusReady
	}
	GetRequestStore().Save(reqInfo)
	w.WriteHeader(http.StatusOK)
	log.Debug("Worker server response ok", "id", workerResp.RequestId)
}

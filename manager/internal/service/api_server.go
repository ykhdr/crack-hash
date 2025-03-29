package service

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"github.com/ykhdr/crack-hash/common/middleware"
	"github.com/ykhdr/crack-hash/manager/config"
	"github.com/ykhdr/crack-hash/manager/pkg/api"
	log "log/slog"
	"net/http"
)

type ApiServer struct {
	addr       string
	dispatcher *Dispatcher
}

func NewApiServer(cfg *config.ManagerConfig, dispatcher *Dispatcher) *ApiServer {
	s := &ApiServer{
		addr:       cfg.ApiServerAddr,
		dispatcher: dispatcher,
	}
	return s
}

func (s *ApiServer) Start() {
	r := mux.NewRouter()
	r.Use(middleware.LoggingMiddleware(log.Debug))
	r.HandleFunc("/api/hash/crack", s.handleHashCrack).Methods("POST")
	r.HandleFunc("/api/hash/status", s.handleHashStatus).Methods("GET")
	r.HandleFunc("/api/health", s.handleHealth).Methods("GET")
	log.Info("Api server is running", "addr", s.addr)
	if err := http.ListenAndServe(s.addr, r); err != nil {
		log.Error("Manager server failed", "err", err)
	}
}

func (s *ApiServer) handleHashCrack(w http.ResponseWriter, r *http.Request) {
	var req api.CrackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Warn("Invalid request", "err", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	reqId, err := DispatchRequest(&req)
	if err != nil {
		log.Warn("Failed to dispatch request", "err", err, "request", req)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
	resp := map[string]string{"requestId": string(reqId)}
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(resp)
	if err != nil {
		log.Warn("Failed to encode response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (s *ApiServer) handleHashStatus(w http.ResponseWriter, r *http.Request) {
	requestId := r.URL.Query().Get("requestId")
	if requestId == "" {
		http.Error(w, "Missing requestId", http.StatusBadRequest)
		return
	}
	id := RequestId(requestId)
	info, exists := GetRequestStore().Get(id)
	if !exists {
		http.Error(w, "Request not found", http.StatusNotFound)
		return
	}
	resp := map[string]interface{}{
		"status": info.Status,
		"data":   []string{},
	}
	if info.Status == StatusReady {
		resp["data"] = info.FoundData
		GetRequestStore().Delete(id)
	}
	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(resp)
	if err != nil {
		log.Warn("Failed to encode response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (s *ApiServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte("OK"))
	if err != nil {
		log.Warn("Failed to write health response", "err", err)
	}
}

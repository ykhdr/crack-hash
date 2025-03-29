package service

import (
	"encoding/xml"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/ykhdr/crack-hash/common"
	"github.com/ykhdr/crack-hash/common/api"
	"github.com/ykhdr/crack-hash/common/consul"
	"github.com/ykhdr/crack-hash/common/middleware"
	"github.com/ykhdr/crack-hash/worker/config"
	"io"
	log "log/slog"
	"net/http"
)

type Server struct {
	cfg          *config.WorkerConfig
	consulClient consul.Client
}

func NewServer(cfg *config.WorkerConfig, consulClient consul.Client) *Server {
	return &Server{
		cfg:          cfg,
		consulClient: consulClient,
	}
}

func (s *Server) Start() {
	err := s.consulClient.RegisterService(common.WorkerService, s.cfg.Address, s.cfg.Port)
	if err != nil {
		log.Warn("Error register service in consul", "err", err)
		return
	}
	r := mux.NewRouter()
	r.Use(middleware.LoggingMiddleware(log.Debug))
	r.HandleFunc("/internal/api/worker/hash/crack/task", s.handleCrackTask).Methods("POST")
	r.HandleFunc("/api/health", s.handleHealth).Methods("GET")
	srv := &http.Server{
		Handler: r,
		Addr:    s.cfg.Url(),
	}
	log.Info("Worker is running", "address", s.cfg.Url())
	if err = srv.ListenAndServe(); err != nil {
		log.Warn("Worker server failed", "err", err)
	}
}

func (s *Server) handleCrackTask(w http.ResponseWriter, r *http.Request) {
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
	go s.crackTask(&reqXml)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Task received. Processing..."))
}

func (s *Server) crackTask(req *api.CrackHashManagerRequest) {
	log.Debug("Start cracking hash", "request", req)
	found := crackMD5(req)
	respXml := api.CrackHashWorkerResponse{
		RequestId: req.RequestId,
		Found:     found,
	}
	s.sendResponseToManager(respXml)
}

// Отправка результата менеджеру
func (s *Server) sendResponseToManager(resp api.CrackHashWorkerResponse) {
	managerURL := fmt.Sprintf("http://%s/internal/api/manager/hash/crack/request", s.cfg.ManagerUrl)
	bytesToSend, err := xml.Marshal(resp)
	if err != nil {
		log.Warn("Failed to marshal response XML", "err", err)
		return
	}
	req, err := http.NewRequest("PATCH", managerURL, io.NopCloser(common.NewBytesReader(bytesToSend)))
	if err != nil {
		log.Warn("Failed to create request", "err", err)
		return
	}
	req.Header.Set("Content-Type", "application/xml")
	client := &http.Client{}
	httpResp, err := client.Do(req)
	if err != nil {
		log.Warn("Failed to send response to manager", "err", err)
		return
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode != http.StatusOK {
		log.Warn("Manager responded with status", "code", httpResp.StatusCode)
		return
	}
	log.Warn("Response successfully sent to manager")
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte("OK"))
	if err != nil {
		log.Warn("Failed to write health response", "err", err)
	}
}

package server

import (
	"context"
	"encoding/xml"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/ykhdr/crack-hash/common/bytes"
	"github.com/ykhdr/crack-hash/common/consul"
	"github.com/ykhdr/crack-hash/common/http/middleware"
	"github.com/ykhdr/crack-hash/manager/pkg/messages"
	"github.com/ykhdr/crack-hash/worker/config"
	"github.com/ykhdr/crack-hash/worker/internal/hashcrack/strategy"
	"github.com/ykhdr/crack-hash/worker/pkg/worker"
	"io"
	"net"
	"net/http"
)

type Server struct {
	l             zerolog.Logger
	cfg           *config.WorkerConfig
	srv           *http.Server
	consulClient  consul.Client
	crackStrategy strategy.Strategy
}

func NewServer(cfg *config.WorkerConfig, l zerolog.Logger, consulClient consul.Client) *Server {
	crackStrategy := strategy.NewStrategy(strategy.ParseStrategyName(cfg.Strategy))
	return &Server{
		cfg:           cfg,
		consulClient:  consulClient,
		crackStrategy: crackStrategy,
		l: l.With().
			Str("domain", "server").
			Str("type", "http").
			Logger(),
	}
}

func (s *Server) Start(ctx context.Context) error {
	err := s.consulClient.RegisterService(worker.ServiceName, s.cfg.Address, s.cfg.Port)
	if err != nil {
		s.l.Warn().Err(err).Msgf("Error register service in consul")
		return errors.Wrap(err, "error register service in consul")
	}
	return s.setup(ctx)
}

func (s *Server) setup(ctx context.Context) error {
	r := mux.NewRouter()
	r.Use(middleware.LoggingMiddleware(s.l))
	r.HandleFunc("/internal/api/worker/hash/crack/task", s.handleCrackTask).Methods("POST")
	r.HandleFunc("/api/health", s.handleHealth).Methods("GET")
	s.srv = &http.Server{
		Handler: r,
		Addr:    s.cfg.Url(),
		BaseContext: func(net.Listener) context.Context {
			return ctx
		},
	}
	s.l.Info().Msgf("worker is running on address: %s", s.cfg.Url())
	if err := s.srv.ListenAndServe(); err != nil {
		s.l.Error().Err(err).Msgf("worker server failed")
		return errors.Wrap(err, "worker server failed")
	}
	s.l.Debug().Msgf("worker server stopped")
	return nil
}

func (s *Server) Shutdown() error {
	return s.srv.Shutdown(context.Background())
}

func (s *Server) handleCrackTask(w http.ResponseWriter, r *http.Request) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	var reqXml messages.CrackHashManagerRequest
	if err := xml.Unmarshal(bodyBytes, &reqXml); err != nil {
		http.Error(w, "Invalid XML", http.StatusBadRequest)
		return
	}
	go s.crackTask(context.WithoutCancel(r.Context()), &reqXml)
	w.WriteHeader(http.StatusOK)
	_, err = w.Write([]byte("Task received. Processing..."))
	if err != nil {
		s.l.Warn().Msgf("failed to write response: %v", err)
	}
}

func (s *Server) crackTask(ctx context.Context, req *messages.CrackHashManagerRequest) {
	s.l.Debug().
		Str("req-id", req.RequestId).
		Int("part-number", req.PartNumber).
		Int("part-count", req.PartCount).
		Str("hash", req.Hash).
		Int("max-length", req.MaxLength).
		Msg("cracking task")
	result := s.crackStrategy.CrackMd5(req)
	respXml := messages.CrackHashWorkerResponse{
		RequestId: req.RequestId,
		Found:     result.Found(),
	}
	s.sendResponseToManager(ctx, respXml)
}

func (s *Server) sendResponseToManager(ctx context.Context, resp messages.CrackHashWorkerResponse) {
	managerURL := fmt.Sprintf("http://%s/internal/api/manager/hash/crack/request", s.cfg.ManagerUrl)
	bytesToSend, err := xml.Marshal(resp)
	if err != nil {
		s.l.Warn().Msgf("failed to marshal response XML: %v", err)
		return
	}
	req, err := http.NewRequestWithContext(ctx, "PATCH", managerURL, io.NopCloser(bytes.NewReader(bytesToSend)))
	if err != nil {
		s.l.Warn().Msgf("failed to create request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/xml")
	client := &http.Client{}
	httpResp, err := client.Do(req)
	if err != nil {
		s.l.Warn().Msgf("failed to send response to manager: %v", err)
		return
	}
	defer func() { _ = httpResp.Body.Close() }()
	if httpResp.StatusCode != http.StatusOK {
		s.l.Warn().Msgf("manager responded with status: %d", httpResp.StatusCode)
		return
	}
	s.l.Debug().Msg("response successfully sent to manager")
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte("OK"))
	if err != nil {
		s.l.Warn().Msgf("failed to write health response: %v", err)
	}
}

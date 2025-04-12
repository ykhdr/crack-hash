package server

import (
	"context"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	amqpconn "github.com/ykhdr/crack-hash/common/amqp/connection"
	"github.com/ykhdr/crack-hash/common/consul"
	"github.com/ykhdr/crack-hash/worker/config"
	"github.com/ykhdr/crack-hash/worker/internal/hashcrack/strategy"
	"net"
	"net/http"
)

type Server struct {
	l             zerolog.Logger
	cfg           *config.WorkerConfig
	srv           *http.Server
	consulClient  consul.Client
	amqpConn      *amqpconn.Connection
	crackStrategy strategy.Strategy
}

func NewServer(cfg *config.WorkerConfig) *Server {
	return &Server{
		cfg: cfg,
		l: log.With().
			Str("domain", "server").
			Str("type", "http").
			Logger(),
	}
}

func (s *Server) Start(ctx context.Context) error {
	if err := s.setupHttpServer(ctx); err != nil {
		s.l.Warn().Err(err).Msgf("Error initialize http server")
		return errors.Wrap(err, "error initialize http server")
	}
	return nil
}

func (s *Server) setupHttpServer(ctx context.Context) error {
	r := mux.NewRouter()
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

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte("OK"))
	if err != nil {
		s.l.Warn().Msgf("failed to write health response: %v", err)
	}
}

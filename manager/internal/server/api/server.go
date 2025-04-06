package api

import (
	"context"
	"encoding/json"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/ykhdr/crack-hash/common/http/middleware"
	"github.com/ykhdr/crack-hash/manager/config"
	"github.com/ykhdr/crack-hash/manager/internal/dispatcher"
	"github.com/ykhdr/crack-hash/manager/internal/messages/request"
	"github.com/ykhdr/crack-hash/manager/internal/store/requeststore"
	"github.com/ykhdr/crack-hash/manager/pkg/api"
	"net"
	"net/http"
)

type Server struct {
	l            zerolog.Logger
	addr         string
	dispatcher   *dispatcher.Dispatcher
	requestStore requeststore.RequestStore
}

func NewServer(
	cfg *config.ManagerConfig,
	dispatcher *dispatcher.Dispatcher,
	requestStore requeststore.RequestStore,
) *Server {
	s := &Server{
		addr:         cfg.ApiServerAddr,
		dispatcher:   dispatcher,
		requestStore: requestStore,
		l: log.With().
			Str("domain", "api-server").
			Str("type", "http").
			Str("content-type", "application/json").
			Logger(),
	}
	return s
}

func (s *Server) Start(ctx context.Context) error {
	router := mux.NewRouter()
	router.Use(middleware.LoggingMiddleware(s.l))
	healthRouter := router.NewRoute().Subrouter()
	router.Use(middleware.ApplicationJsonContentTypeMiddleware())
	router.HandleFunc("/api/hash/crack", s.handleHashCrack).Methods("POST")
	router.HandleFunc("/api/hash/status", s.handleHashStatus).Methods("GET")
	healthRouter.HandleFunc("/api/health", s.handleHealth).Methods("GET")
	s.l.Info().Str("address", s.addr).Msgf("Api server is running")
	server := http.Server{
		Addr:    s.addr,
		Handler: router,
		BaseContext: func(listener net.Listener) context.Context {
			return ctx
		},
	}
	if err := server.ListenAndServe(); err != nil {
		s.l.Error().Err(err).Msg("Manager server failed")
		return err
	}
	return nil
}

func (s *Server) handleHashCrack(w http.ResponseWriter, r *http.Request) {
	var req api.CrackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.l.Warn().Err(err).Msg("Invalid request")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	reqId, err := s.dispatcher.DispatchRequest(&req)
	if err != nil {
		s.l.Warn().Err(err).Any("request", req).Msg("Failed to dispatch request")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	resp := map[string]string{"requestId": string(reqId)}
	err = json.NewEncoder(w).Encode(resp)
	if err != nil {
		s.l.Warn().Err(err).Msg("Failed to encode response")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (s *Server) handleHashStatus(w http.ResponseWriter, r *http.Request) {
	requestId := r.URL.Query().Get("requestId")
	if requestId == "" {
		http.Error(w, "Missing requestId", http.StatusBadRequest)
		return
	}
	id := request.Id(requestId)
	info, exists := s.requestStore.Get(id)
	if !exists {
		http.Error(w, "Request not found", http.StatusNotFound)
		return
	}
	resp := map[string]interface{}{
		"status": info.Status,
		"data":   []string{},
	}
	if info.Status == request.StatusReady {
		resp["data"] = info.FoundData
		s.requestStore.Delete(id)
	}
	err := json.NewEncoder(w).Encode(resp)
	if err != nil {
		s.l.Warn().Err(err).Msg("Failed to encode response")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte("OK"))
	if err != nil {
		s.l.Warn().Err(err).Msg("Failed to write health response")
	}
}

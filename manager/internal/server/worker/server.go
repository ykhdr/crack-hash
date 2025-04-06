package worker

import (
	"context"
	"encoding/xml"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/ykhdr/crack-hash/common/http/middleware"
	"github.com/ykhdr/crack-hash/manager/config"
	"github.com/ykhdr/crack-hash/manager/internal/messages/request"
	"github.com/ykhdr/crack-hash/manager/internal/store/requeststore"
	"github.com/ykhdr/crack-hash/manager/pkg/messages"
	"io"
	"net"
	"net/http"
)

type Server struct {
	l            zerolog.Logger
	addr         string
	requestStore requeststore.RequestStore
}

func NewServer(cfg *config.ManagerConfig, requestStore requeststore.RequestStore) *Server {
	return &Server{
		addr:         cfg.WorkerServerAddr,
		requestStore: requestStore,
		l: log.With().
			Str("domain", "worker-server").
			Str("type", "http").
			Str("content-type", "application/xml").
			Logger(),
	}
}

func (s *Server) Start(ctx context.Context) error {
	r := mux.NewRouter()
	r.HandleFunc("/internal/api/manager/hash/crack/request", s.handleWorkerResponse).Methods("PATCH")
	r.Use(
		middleware.LoggingMiddleware(s.l),
		middleware.ApplicationXmlContentTypeMiddleware(),
	)
	s.l.Info().Str("address", s.addr).Msg("Worker server is running")
	server := http.Server{
		Addr:    s.addr,
		Handler: r,
		BaseContext: func(listener net.Listener) context.Context {
			return ctx
		},
	}
	if err := server.ListenAndServe(); err != nil {
		s.l.Error().Err(err).Msg("Worker server failed")
		return err
	}
	return nil
}

func (s *Server) handleWorkerResponse(w http.ResponseWriter, r *http.Request) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		s.l.Warn().Err(err).Msg("Failed to read worker response body")
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	var workerResp messages.CrackHashWorkerResponse
	if err = xml.Unmarshal(bodyBytes, &workerResp); err != nil {
		s.l.Warn().Err(err).Msg("Failed to unmarshal worker response body")
		http.Error(w, "Invalid XML", http.StatusBadRequest)
		return
	}
	reqInfo, exists := s.requestStore.Get(request.Id(workerResp.RequestId))
	if !exists {
		s.l.Warn().Str("request-id", workerResp.RequestId).Msg("Failed to find request store")
		http.Error(w, "Request not found", http.StatusNotFound)
		return
	}
	if reqInfo.Status == request.StatusError || reqInfo.ReadyServiceCount+reqInfo.FailedServiceCount == reqInfo.ServiceCount {
		s.l.Warn().Str("request-id", workerResp.RequestId).Msg("Request is already canceled")
		http.Error(w, "Request failed", http.StatusInternalServerError)
		return
	}
	reqInfo.FoundData = append(reqInfo.FoundData, workerResp.Found...)
	reqInfo.ReadyServiceCount++
	reqInfo.UpdateStatus()
	s.requestStore.Save(reqInfo)
	w.WriteHeader(http.StatusOK)
	s.l.Debug().Str("request-id", workerResp.RequestId).Msg("Worker response is ok")
}

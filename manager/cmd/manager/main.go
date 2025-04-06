package main

import (
	"context"
	"github.com/rs/zerolog/log"
	"github.com/ykhdr/crack-hash/common/amqp"
	"github.com/ykhdr/crack-hash/common/consul"
	"github.com/ykhdr/crack-hash/manager/config"
	"github.com/ykhdr/crack-hash/manager/internal/dispatcher"
	"github.com/ykhdr/crack-hash/manager/internal/server/api"
	"github.com/ykhdr/crack-hash/manager/internal/server/worker"
	"github.com/ykhdr/crack-hash/manager/internal/store/requeststore"
	"golang.org/x/sync/errgroup"
	"os"
)

func main() {
	cfg, err := config.InitializeConfig(os.Args[1:])
	if err != nil {
		log.Fatal().Err(err).Msgf("Error initializing config")
	}
	consulClient, err := consul.NewClient(cfg.ConsulConfig)
	if err != nil {
		log.Fatal().Err(err).Msgf("Error initializing consul client")
	}
	group, gCtx := errgroup.WithContext(context.Background())
	requestStore := requeststore.NewRequestStore()
	amqpConn, err := amqp.Dial(gCtx, cfg.AmqpConfig)
	if err != nil {
		log.Fatal().Err(err).Msgf("Error initializing amqp connection")
	}

	dispatcherSrv := dispatcher.NewDispatcher(cfg.DispatcherConfig, cfg.AmqpConfig, consulClient, requestStore, amqpConn)
	apiSrv := api.NewServer(cfg, dispatcherSrv, requestStore)
	workerSrv := worker.NewServer(cfg, requestStore)

	group.Go(func() error {
		return dispatcherSrv.Start(gCtx)
	})
	group.Go(func() error {
		return apiSrv.Start(gCtx)
	})
	group.Go(func() error {
		return workerSrv.Start(gCtx)
	})
	if err = group.Wait(); err != nil {
		log.Error().Err(err).Msgf("Manager failed")
	}
}

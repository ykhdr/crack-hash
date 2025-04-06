package main

import (
	"context"
	"github.com/rs/zerolog/log"
	"github.com/ykhdr/crack-hash/common/amqp"
	"github.com/ykhdr/crack-hash/common/consul"
	"github.com/ykhdr/crack-hash/worker/config"
	"github.com/ykhdr/crack-hash/worker/internal/hashcrack"
	"github.com/ykhdr/crack-hash/worker/internal/server"
	"golang.org/x/sync/errgroup"
	"os"
)

func main() {
	cfg, err := config.InitializeConfig(os.Args[1:])
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed to initialize config")
	}
	consulClient, err := consul.NewClient(cfg.ConsulConfig)
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed to initialize consul client")
	}
	group, gCtx := errgroup.WithContext(context.Background())
	amqpConn, err := amqp.Dial(gCtx, cfg.AmqpConfig)
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed to initialize amqp connection")
	}
	srv := server.NewServer(cfg)
	hashCrackSrv := hashcrack.NewService(cfg, consulClient, amqpConn)
	group.Go(func() error {
		return srv.Start(gCtx)
	})
	group.Go(func() error {
		return hashCrackSrv.Start(gCtx)
	})
	if err = group.Wait(); err != nil {
		log.Fatal().Err(err).Msgf("Worker failed")
	}
}

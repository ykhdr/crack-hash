package main

import (
	"context"
	"github.com/rs/zerolog/log"
	"github.com/ykhdr/crack-hash/common/consul"
	"github.com/ykhdr/crack-hash/worker/config"
	"github.com/ykhdr/crack-hash/worker/internal/server"
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
	srv := server.NewServer(cfg, log.Logger, consulClient)
	if err = srv.Start(context.Background()); err != nil {
		log.Fatal().Err(err).Msgf("Server failed")
	}
}

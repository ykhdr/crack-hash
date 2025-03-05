package main

import (
	"github.com/ykhdr/crack-hash/common/consul"
	"github.com/ykhdr/crack-hash/worker/config"
	_ "github.com/ykhdr/crack-hash/worker/logger"
	"github.com/ykhdr/crack-hash/worker/service"
	log "log/slog"
	"os"
)

func main() {
	cfg, err := config.InitializeConfig(os.Args[1:])
	if err != nil {
		log.Error("Failed to initialize config", "err", err)
		return
	}
	consulClient, err := consul.NewClient(cfg.ConsulConfig)
	if err != nil {
		log.Error("Failed to initialize consul client", "err", err)
		return
	}
	srv := service.NewServer(cfg, consulClient)
	srv.Start()
}

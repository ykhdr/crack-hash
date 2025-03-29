package main

import (
	"github.com/ykhdr/crack-hash/common"
	"github.com/ykhdr/crack-hash/common/consul"
	"github.com/ykhdr/crack-hash/manager/config"
	_ "github.com/ykhdr/crack-hash/manager/internal/logger"
	service2 "github.com/ykhdr/crack-hash/manager/internal/service"
	log "log/slog"
	"os"
)

func main() {
	cfg, err := config.InitializeConfig(os.Args[1:])
	if err != nil {
		log.Warn("Error initializing config", "err", err)
		return
	}

	consulClient, err := consul.NewClient(cfg.ConsulConfig)
	if err != nil {
		log.Warn("Error initializing consulClient", "err", err)
		return
	}
	dispatcherSrv := service2.NewDispatcher(cfg.DispatcherConfig, consulClient)
	apiSrv := service2.NewApiServer(cfg, dispatcherSrv)
	workerSrv := service2.NewWorkerServer(cfg)

	servers := []common.Server{
		dispatcherSrv,
		apiSrv,
		workerSrv,
	}

	for _, server := range servers {
		go server.Start()
	}
	select {}
}

package main

import (
	"github.com/metal-stack/bmc-catcher/domain"
	"github.com/metal-stack/bmc-catcher/internal/leases"
	"github.com/metal-stack/bmc-catcher/internal/sel"
	metalgo "github.com/metal-stack/metal-go"
	"github.com/metal-stack/v"

	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewProduction()
	log := logger.Sugar()
	log.Infow("running app version", "version", v.V.String())
	var cfg domain.Config
	if err := envconfig.Process("BMC_CATCHER", &cfg); err != nil {
		log.Fatalw("bad configuration", "error", err)
	}

	log.Infow("loaded configuration", "config", cfg)

	driver, err := metalgo.NewDriver(cfg.MetalAPIURL.String(), "", cfg.MetalAPIHMACKey, metalgo.AuthType("Metal-Edit"))
	if err != nil {
		log.Fatalw("could not create metal-go driver", "error", err)
	}

	r, err := leases.NewReporter(&cfg, driver, log)
	if err != nil {
		log.Fatalw("could not start reporter", "error", err)
	}

	s := sel.New(&cfg, driver, log)

	go s.Run()

	lr := leases.New(&cfg, r, log)
	lr.Run() // Blocking call

}

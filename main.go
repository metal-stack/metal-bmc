package main

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/metal-stack/metal-bmc/internal/bmc"
	"github.com/metal-stack/metal-bmc/pkg/config"
	metalgo "github.com/metal-stack/metal-go"

	"github.com/metal-stack/metal-bmc/internal/reporter"
	"github.com/metal-stack/v"

	"github.com/kelseyhightower/envconfig"
)

func main() {
	var cfg config.Config
	if err := envconfig.Process("METAL_BMC", &cfg); err != nil {
		panic(fmt.Errorf("bad configuration: %w", err))
	}

	if err := cfg.Validate(); err != nil {
		panic(fmt.Errorf("bad configuration: %w", err))
	}

	level := slog.LevelInfo
	switch strings.ToLower(cfg.LogLevel) {
	case "debug":
		level = slog.LevelDebug
	case "error":
		level = slog.LevelError
	case "warn":
		level = slog.LevelWarn
	}

	jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})
	log := slog.New(jsonHandler)

	log.Info("running app version", "version", v.V.String())
	log.Info("configuration", "config", cfg)

	client, err := metalgo.NewDriver(cfg.MetalAPIURL.String(), "", cfg.MetalAPIHMACKey, metalgo.AuthType("Metal-Edit"))
	if err != nil {
		log.Error("unable to create metal-api client", "error", err)
		panic(err)
	}

	// BMC Events via NSQ
	b := bmc.New(log, &cfg)

	err = b.InitConsumer()
	if err != nil {
		log.Error("unable to create bmc service", "error", err)
		panic(err)
	}

	// BMC Console access
	console, err := bmc.NewConsole(log, client, cfg)
	if err != nil {
		log.Error("unable to create bmc console", "error", err)
		panic(err)
	}
	go func() {
		err := console.ListenAndServe()
		if err != nil {
			panic(err)
		}
	}()

	// Report IPMI Details
	r, err := reporter.New(log, &cfg, client)
	if err != nil {
		log.Error("could not start reporter", "error", err)
		panic(err)
	}

	r.Run()
}

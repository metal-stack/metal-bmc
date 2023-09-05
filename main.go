package main

import (
	"fmt"

	"github.com/metal-stack/metal-bmc/internal/bmc"
	"github.com/metal-stack/metal-bmc/pkg/config"
	metalgo "github.com/metal-stack/metal-go"

	"github.com/metal-stack/metal-bmc/internal/reporter"
	"github.com/metal-stack/v"

	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	var cfg config.Config
	if err := envconfig.Process("METAL_BMC", &cfg); err != nil {
		panic(fmt.Errorf("bad configuration: %w", err))
	}

	if err := cfg.Validate(); err != nil {
		panic(fmt.Errorf("bad configuration: %w", err))
	}

	level, err := zap.ParseAtomicLevel(cfg.LogLevel)
	if err != nil {
		panic(fmt.Errorf("can't initialize zap logger: %w", err))
	}

	zcfg := zap.NewProductionConfig()
	zcfg.EncoderConfig.TimeKey = "timestamp"
	zcfg.EncoderConfig.EncodeTime = zapcore.RFC3339TimeEncoder
	zcfg.Level = level

	l, err := zcfg.Build()
	if err != nil {
		panic(fmt.Errorf("can't initialize zap logger: %w", err))
	}

	log := l.Sugar()
	log.Infow("running app version", "version", v.V.String())
	log.Infow("configuration", "config", cfg)

	client, err := metalgo.NewDriver(cfg.MetalAPIURL.String(), "", cfg.MetalAPIHMACKey, metalgo.AuthType("Metal-Edit"))
	if err != nil {
		log.Fatalw("unable to create metal-api client", "error", err)
	}

	// BMC Events via NSQ
	b := bmc.New(bmc.Config{
		Log:              log,
		MQAddress:        cfg.MQAddress,
		MQCACertFile:     cfg.MQCACertFile,
		MQClientCertFile: cfg.MQClientCertFile,
		MQLogLevel:       cfg.MQLogLevel,
		MachineTopic:     cfg.MachineTopic,
		MachineTopicTTL:  cfg.MachineTopicTTL,
	})

	err = b.InitConsumer()
	if err != nil {
		log.Fatalw("unable to create bmc service", "error", err)
	}

	// BMC Console access
	console, err := bmc.NewConsole(log, client, cfg.ConsoleCACertFile, cfg.ConsoleCertFile, cfg.ConsoleKeyFile, cfg.ConsolePort)
	if err != nil {
		log.Fatalw("unable to create bmc console", "error", err)
	}
	go func() {
		log.Fatal(console.ListenAndServe())
	}()

	// Report IPMI Details
	r, err := reporter.New(log, &cfg, client)
	if err != nil {
		log.Fatalw("could not start reporter", "error", err)
	}

	r.Run()
}

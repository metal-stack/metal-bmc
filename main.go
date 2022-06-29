package main

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/metal-stack/bmc-catcher/domain"
	"github.com/metal-stack/bmc-catcher/internal/bmc"
	"github.com/metal-stack/bmc-catcher/internal/leases"
	"github.com/metal-stack/bmc-catcher/internal/reporter"
	"github.com/metal-stack/v"

	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	var cfg domain.Config
	if err := envconfig.Process("BMC_CATCHER", &cfg); err != nil {
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
		log.Fatalw("unable to create bmcservice", "error", err)
	}

	r, err := reporter.NewReporter(&cfg, log)
	if err != nil {
		log.Fatalw("could not start reporter", "error", err)
	}

	periodic := time.NewTicker(cfg.ReportInterval)
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-periodic.C:
			ls, err := leases.ReadLeases(cfg.LeaseFile)
			if err != nil {
				log.Fatalw("could not parse leases file", "error", err)
			}
			active := ls.FilterActive()
			byMac := active.LatestByMac()
			log.Infow("reporting leases to metal-api", "all", len(ls), "active", len(active), "uniqueActive", len(byMac))

			mtx := new(sync.Mutex)
			var items []*leases.ReportItem

			wg := new(sync.WaitGroup)
			wg.Add(len(byMac))

			for _, l := range byMac {
				item := leases.NewReportItem(l, log)
				go func() {
					item.EnrichWithBMCDetails(cfg.IpmiPort, cfg.IpmiUser, cfg.IpmiPassword)
					mtx.Lock()
					items = append(items, item)
					wg.Done()
					mtx.Unlock()
				}()
			}

			wg.Wait()

			err = r.Report(items)
			if err != nil {
				log.Warnw("could not report ipmi addresses", "error", err)
			}
		case <-signals:
			return
		}
	}
}

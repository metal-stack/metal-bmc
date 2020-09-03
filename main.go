package main

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/metal-stack/ipmi-catcher/domain"
	"github.com/metal-stack/ipmi-catcher/internal/ipmi"
	"github.com/metal-stack/ipmi-catcher/internal/leases"
	"github.com/metal-stack/ipmi-catcher/internal/reporter"
	"github.com/metal-stack/v"

	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	"go.universe.tf/netboot/dhcp4"
)

func main() {
	logger, _ := zap.NewProduction()
	log := logger.Sugar()
	log.Infow("running app version", "version", v.V.String())
	var cfg domain.Config
	if err := envconfig.Process("IPMI_CATCHER", &cfg); err != nil {
		log.Fatalw("bad configuration", "error", err)
	}

	log.Infow("loaded configuration", "config", cfg)
	l, err := leases.ReadLeases(cfg.LeaseFile)
	if err != nil {
		log.Fatalw("could not parse leases file", "error", err)
	}

	log.Info("warming up cache")
	leasesByMac := l.LatestByMac()
	macToIps := map[string]string{}
	for m, l := range leasesByMac {
		macToIps[m] = l.Ip
	}
	uuidCache := ipmi.NewUUIDCache(cfg.IpmiPort, cfg.IpmiUser, cfg.IpmiPassword)
	uuidCache.Warmup(macToIps)

	r, err := reporter.NewReporter(&cfg, &uuidCache, log)
	if err != nil {
		log.Fatalw("could not start reporter", "error", err)
	}
	err = r.Report(l)
	if err != nil {
		log.Fatalw("could not send initial report of ipmi addresses", "error", err)
	}

	periodic := time.NewTicker(cfg.ReportInterval)
	dhcpEvents, err := snoopDhcpEvents()
	if err != nil {
		log.Fatalw("could not initialize dhcp snooper", "error", err)
	}
	debounced := time.NewTimer(cfg.DebounceInterval)
	debounced.Stop()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

outer:
	for {
		select {
		case <-periodic.C:
			debounced.Reset(cfg.DebounceInterval)
		case <-dhcpEvents:
			debounced.Reset(cfg.DebounceInterval)
		case <-debounced.C:
			l, err := leases.ReadLeases(cfg.LeaseFile)
			if err != nil {
				log.Fatalw("could not parse leases file", "error", err)
			}
			err = r.Report(l)
			if err != nil {
				log.Warnw("could not report ipmi addresses", "error", err)
			}
		case <-signals:
			break outer
		}
	}
}

func snoopDhcpEvents() (chan dhcp4.Packet, error) {
	c := make(chan dhcp4.Packet, 10)
	dhcp, err := dhcp4.NewSnooperConn("0.0.0.0:67")
	if err != nil {
		return nil, err
	}
	go func() {
		for {
			p, _, err := dhcp.RecvDHCP()
			if err == nil {
				c <- *p
			}
		}
	}()
	return c, nil
}

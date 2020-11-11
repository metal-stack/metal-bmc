package main

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/metal-stack/bmc-catcher/domain"
	"github.com/metal-stack/bmc-catcher/internal/leases"
	"github.com/metal-stack/bmc-catcher/internal/reporter"
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
	if err := envconfig.Process("BMC_CATCHER", &cfg); err != nil {
		log.Fatalw("bad configuration", "error", err)
	}

	log.Infow("loaded configuration", "config", cfg)

	r, err := reporter.NewReporter(&cfg, log, cfg.IpmiPort, cfg.IpmiUser, cfg.IpmiPassword)
	if err != nil {
		log.Fatalw("could not start reporter", "error", err)
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
				item := leases.NewReportItem(l, cfg, log)
				go func() {
					item.EnrichWithBMCDetails()
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

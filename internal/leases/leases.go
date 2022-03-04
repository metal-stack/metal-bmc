package leases

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/metal-stack/bmc-catcher/domain"
	"go.uber.org/zap"
)

type leasRunner struct {
	cfg      *domain.Config
	reporter *Reporter
	log      *zap.SugaredLogger
}

func New(cfg *domain.Config, reporter *Reporter, log *zap.SugaredLogger) *leasRunner {
	return &leasRunner{
		cfg:      cfg,
		reporter: reporter,
		log:      log,
	}
}

func (lr *leasRunner) Run() {
	periodic := time.NewTicker(lr.cfg.ReportInterval)
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-periodic.C:
			ls, err := ReadLeases(lr.cfg.LeaseFile)
			if err != nil {
				lr.log.Fatalw("could not parse leases file", "error", err)
			}
			active := ls.FilterActive()
			byMac := active.LatestByMac()
			lr.log.Infow("reporting leases to metal-api", "all", len(ls), "active", len(active), "uniqueActive", len(byMac))

			mtx := new(sync.Mutex)
			var items []*ReportItem

			wg := new(sync.WaitGroup)
			wg.Add(len(byMac))

			for _, l := range byMac {
				item := NewReportItem(l, lr.log)
				go func() {
					item.EnrichWithBMCDetails(lr.cfg.IpmiPort, lr.cfg.IpmiUser, lr.cfg.IpmiPassword)
					mtx.Lock()
					items = append(items, item)
					wg.Done()
					mtx.Unlock()
				}()
			}

			wg.Wait()

			err = lr.reporter.Report(items)
			if err != nil {
				lr.log.Warnw("could not report ipmi addresses", "error", err)
			}
		case <-signals:
			return
		}
	}
}

func (l Leases) FilterActive() Leases {
	active := Leases{}
	now := time.Now()
	for _, lease := range l {
		if lease.End.Before(now) {
			continue
		}
		active = append(active, lease)
	}
	return active
}

func (l Leases) LatestByMac() map[string]Lease {
	byMac := map[string]Lease{}
	for _, lease := range l {
		if e, ok := byMac[lease.Mac]; !ok {
			byMac[lease.Mac] = lease
		} else if lease.End.After(e.End) {
			byMac[lease.Mac] = lease
		}
	}
	return byMac
}

func ReadLeases(leaseFile string) (Leases, error) {
	leasesContent := mustRead(leaseFile)
	leases, err := Parse(leasesContent)
	if err != nil {
		return nil, fmt.Errorf("could not parse leases file:%w", err)
	}
	return leases, nil
}

func mustRead(name string) string {
	c, err := os.ReadFile(name)
	if err != nil {
		panic(err)
	}
	return string(c)
}

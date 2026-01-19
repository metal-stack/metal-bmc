package reporter

import (
	"context"
	"fmt"
	"log/slog"
	"net/netip"
	"os"
	"os/signal"
	"slices"
	"syscall"
	"time"

	apiclient "github.com/metal-stack/api/go/client"
	apiv2 "github.com/metal-stack/api/go/metalstack/api/v2"
	infrav2 "github.com/metal-stack/api/go/metalstack/infra/v2"
	"github.com/metal-stack/metal-lib/pkg/pointer"

	"github.com/metal-stack/metal-bmc/internal/leases"
	"github.com/metal-stack/metal-bmc/pkg/config"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

// reporter reports information about bmc, bios and dhcp ip of bmc to metal-api
type reporter struct {
	cfg    *config.Config
	log    *slog.Logger
	client apiclient.Client
	sem    *semaphore.Weighted
}

// New will create a reporter for MachineIpmiReports
func New(log *slog.Logger, cfg *config.Config, client apiclient.Client) (*reporter, error) {
	return &reporter{
		cfg:    cfg,
		log:    log,
		client: client,
		sem:    semaphore.NewWeighted(1),
	}, nil
}

func (r reporter) Run() {
	periodic := time.NewTicker(r.cfg.ReportInterval)
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	for {
		select {
		case <-periodic.C:
			err := r.collectAndReport()
			if err != nil {
				r.log.Error("collect and report", "error", err)
			}
		case <-signals:
			return
		}
	}
}

func (r reporter) collectAndReport() error {
	if !r.sem.TryAcquire(1) {
		r.log.Warn("lease reporting is still running")
		return nil
	}
	defer r.sem.Release(1)

	start := time.Now()

	items, err := r.getReportItems()
	if err != nil {
		return fmt.Errorf("unable to retrieve report items: %w", err)
	}

	r.log.Info("reporting leases to metal-api", "count", len(items))

	g := new(errgroup.Group)
	// Allow 20 goroutines run in parallel at max
	g.SetLimit(20)
	for _, item := range items {
		g.Go(func() error {
			return item.EnrichWithBMCDetails(r.log, r.cfg.BMCPort, r.cfg.BMCUser, r.cfg.BMCPassword)
		})
	}
	err = g.Wait()
	if err != nil {
		r.log.Error("could not enrich all bmc details", "error", err)
	}

	err = r.report(items)
	if err != nil {
		return fmt.Errorf("could not report bmc addresses %w", err)
	}
	r.log.Info("reporting leases to metal-api", "took", time.Since(start).String())
	return nil
}

func (r reporter) getReportItems() ([]*leases.ReportItem, error) {
	ls, err := leases.ReadLeases(r.log, r.cfg.LeaseFile)
	if err != nil {
		return nil, err
	}

	if len(ls) == 0 {
		r.log.Warn("empty leases returned, nothing to report")
		return nil, nil
	}

	active := ls.FilterActive()
	byMac := active.LatestByMac()

	r.log.Info("consider reporting leases to metal-api", "all", len(ls), "active", len(active), "uniqueActive", len(byMac))

	var items []*leases.ReportItem
	for _, l := range byMac {
		if !r.isInAllowedCidr(l.Ip) {
			continue
		}

		if slices.Contains(r.cfg.IgnoreMacs, l.Mac) {
			continue
		}

		item := &leases.ReportItem{
			Lease: l,
		}
		items = append(items, item)
	}

	return items, nil
}

func (r reporter) isInAllowedCidr(ip string) bool {
	parsedIP, err := netip.ParseAddr(ip)
	if err != nil {
		r.log.Error("given ip is not parsable", "ip", ip, "error", err)
		return false
	}
	for _, cidr := range r.cfg.AllowedCidrs {
		cidr := cidr
		pfx, err := netip.ParsePrefix(cidr)
		if err != nil {
			return false
		}
		if pfx.Contains(parsedIP) {
			return true
		}
	}
	return false
}

// report will send all gathered information about machines to the metal-api
func (r reporter) report(items []*leases.ReportItem) error {
	partitionID := r.cfg.PartitionID
	reports := make(map[string]*apiv2.MachineBMCReport)

	for _, item := range items {
		item := item
		if item.UUID == nil {
			r.log.Error("could not determine uuid of device", "mac", item.Lease.Mac, "ip", item.Lease.Ip)
			continue
		}

		report := &apiv2.MachineBMCReport{
			Bmc: &apiv2.MachineBMC{
				// FIXME
				Address:    item.Lease.Ip + ":631",
				Version:    pointer.SafeDeref(item.BmcVersion),
				PowerState: pointer.SafeDeref(item.Powerstate),
			},
			Bios: &apiv2.MachineBios{
				Version: pointer.SafeDeref(item.BiosVersion),
			},
			Fru:           item.FRU,
			PowerMetric:   item.PowerMetric,
			LedState:      &apiv2.MachineChassisIdentifyLEDState{Value: pointer.SafeDeref(item.IndicatorLED)},
			PowerSupplies: item.PowerSupplies,
		}
		reports[*item.UUID] = report
	}

	ok, err := r.client.Infrav2().BMC().UpdateBMCInfo(context.Background(), &infrav2.UpdateBMCInfoRequest{Partition: partitionID, BmcReports: reports})
	if err != nil {
		return err
	}

	r.log.Info("updated bmc information", "# of machines", len(ok.UpdatedMachines))
	for _, u := range ok.UpdatedMachines {
		r.log.Info("bmc information was updated for machine", "uuid", u)
	}
	for _, u := range ok.CreatedMachines {
		r.log.Info("bmc information was set and machine was created", "uuid", u)
	}

	return nil
}

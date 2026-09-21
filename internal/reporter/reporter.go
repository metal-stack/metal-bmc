package reporter

import (
	"context"
	"fmt"
	"log/slog"
	"net/netip"
	"os/signal"
	"slices"
	"syscall"
	"time"

	"github.com/metal-stack/api/go/client"
	apiv2 "github.com/metal-stack/api/go/metalstack/api/v2"
	infrav2 "github.com/metal-stack/api/go/metalstack/infra/v2"
	"github.com/metal-stack/metal-bmc/internal/leases"
	"github.com/metal-stack/metal-bmc/pkg/config"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

// reporter reports information about bmc, bios and dhcp ip of bmc to metal-api
type reporter struct {
	cfg    *config.Config
	log    *slog.Logger
	client client.Client
	sem    *semaphore.Weighted
}

// New will create a reporter for MachineIpmiReports
func New(log *slog.Logger, cfg *config.Config, client client.Client) (*reporter, error) {
	return &reporter{
		cfg:    cfg,
		log:    log,
		client: client,
		sem:    semaphore.NewWeighted(1),
	}, nil
}

func (r reporter) Run() {
	periodic := time.NewTicker(r.cfg.ReportInterval)
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	for {
		select {
		case <-periodic.C:
			err := r.collectAndReport(ctx)
			if err != nil {
				r.log.Error("collect and report", "error", err)
			}
		case <-ctx.Done():
			r.log.Info("received shutdown signal, stopping")
			return
		}
	}
}

func (r reporter) collectAndReport(ctx context.Context) error {
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
			return item.EnrichWithBMCDetails(r.log, r.cfg.IpmiPort, r.cfg.IpmiUser, r.cfg.IpmiPassword)
		})
	}
	err = g.Wait()
	if err != nil {
		r.log.Error("could not enrich all ipmi details", "error", err)
	}

	err = r.report(ctx, items)
	if err != nil {
		return fmt.Errorf("could not report ipmi addresses %w", err)
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
func (r reporter) report(ctx context.Context, items []*leases.ReportItem) error {
	var reports []*apiv2.MachineBMCReport

	for _, item := range items {
		if item.Uuid == "" {
			r.log.Error("could not determine uuid of device", "mac", item.Lease.Mac, "ip", item.Lease.Ip)
			continue
		}

		reports = append(reports, &item.MachineBMCReport)
	}

	resp, err := r.client.Infrav2().BMC().UpdateBMCInfo(ctx, &infrav2.UpdateBMCInfoRequest{
		Partition:  r.cfg.PartitionID,
		BmcReports: reports,
	})
	if err != nil {
		return err
	}

	r.log.Info("updated ipmi information", "# of machines", len(resp.UpdatedMachines))
	for _, u := range resp.UpdatedMachines {
		r.log.Info("ipmi information was updated for machine", "uuid", u)
	}
	for _, u := range resp.CreatedMachines {
		r.log.Info("ipmi information was set and machine was created", "uuid", u)
	}

	return nil
}

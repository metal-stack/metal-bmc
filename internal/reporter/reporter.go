package reporter

import (
	"net/netip"
	"os"
	"os/signal"
	"slices"
	"syscall"
	"time"

	"github.com/metal-stack/metal-bmc/internal/leases"
	"github.com/metal-stack/metal-bmc/pkg/config"
	metalgo "github.com/metal-stack/metal-go"
	"github.com/metal-stack/metal-go/api/client/machine"
	"github.com/metal-stack/metal-go/api/models"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// reporter reports information about bmc, bios and dhcp ip of bmc to metal-api
type reporter struct {
	cfg    *config.Config
	log    *zap.SugaredLogger
	client metalgo.Client
}

// New will create a reporter for MachineIpmiReports
func New(log *zap.SugaredLogger, cfg *config.Config, client metalgo.Client) (*reporter, error) {
	return &reporter{
		cfg:    cfg,
		log:    log,
		client: client,
	}, nil
}

func (r reporter) Run() {
	periodic := time.NewTicker(r.cfg.ReportInterval)
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-periodic.C:
			ls, err := leases.ReadLeases(r.cfg.LeaseFile)
			if err != nil {
				r.log.Errorw("could not parse leases file", "error", err)
			}
			if len(ls) == 0 {
				r.log.Errorw("empty leases returned, nothing to report")
				continue
			}
			active := ls.FilterActive()
			byMac := active.LatestByMac()
			r.log.Infow("consider reporting leases to metal-api", "all", len(ls), "active", len(active), "uniqueActive", len(byMac))

			var items []*leases.ReportItem
			g := new(errgroup.Group)
			g.SetLimit(20)

			for _, l := range byMac {
				if !r.isInAllowedCidr(l.Ip) {
					continue
				}

				if slices.Contains(r.cfg.IgnoreMacs, l.Mac) {
					continue
				}

				item := &leases.ReportItem{
					Lease: l,
					Log:   r.log,
				}
				items = append(items, item)
			}
			r.log.Infow("reporting leases to metal-api", "count", len(items))

			for _, item := range items {
				item := item
				g.Go(func() error {
					item.EnrichWithBMCDetails(r.cfg.IpmiPort, r.cfg.IpmiUser, r.cfg.IpmiPassword)
					return nil
				})
			}
			err = g.Wait()
			if err != nil {
				r.log.Errorw("could not collect ipmi details", "error", err)
			}

			err = r.report(items)
			if err != nil {
				r.log.Warnw("could not report ipmi addresses", "error", err)
			}
		case <-signals:
			return
		}
	}
}

func (r reporter) isInAllowedCidr(ip string) bool {
	parsedIP, err := netip.ParseAddr(ip)
	if err != nil {
		r.log.Errorw("given ip is not parsable", "ip", ip, "error", err)
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
	reports := make(map[string]models.V1MachineIpmiReport)

	for _, item := range items {
		item := item
		if item.UUID == nil {
			r.log.Errorw("could not determine uuid of device", "mac", item.Mac, "ip", item.Ip)
			continue
		}

		report := models.V1MachineIpmiReport{
			BMCIP:             &item.Ip,
			BMCVersion:        item.BmcVersion,
			BIOSVersion:       item.BiosVersion,
			FRU:               item.FRU,
			PowerState:        item.Powerstate,
			IndicatorLEDState: item.IndicatorLED,
			PowerMetric:       item.PowerMetric,
		}
		reports[*item.UUID] = report
	}

	mir := &models.V1MachineIpmiReports{
		Partitionid: partitionID,
		Reports:     reports,
	}

	ok, err := r.client.Machine().IpmiReport(machine.NewIpmiReportParams().WithBody(mir), nil)
	if err != nil {
		return err
	}

	r.log.Infof("updated ipmi information of %d machines", len(ok.Payload.Updated))
	for _, u := range ok.Payload.Updated {
		r.log.Infow("ipmi information was updated for machine", "uuid", u)
	}
	for _, u := range ok.Payload.Created {
		r.log.Infow("ipmi information was set and machine was created", "uuid", u)
	}

	return nil
}

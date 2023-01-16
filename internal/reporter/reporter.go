package reporter

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/metal-stack/metal-bmc/domain"
	"github.com/metal-stack/metal-bmc/internal/leases"
	metalgo "github.com/metal-stack/metal-go"
	"github.com/metal-stack/metal-go/api/client/machine"
	"github.com/metal-stack/metal-go/api/models"
	"go.uber.org/zap"
)

// reporter reports information about bmc, bios and dhcp ip of bmc to metal-api
type reporter struct {
	cfg    *domain.Config
	log    *zap.SugaredLogger
	client metalgo.Client
}

// New will create a reporter for MachineIpmiReports
func New(log *zap.SugaredLogger, cfg *domain.Config, client metalgo.Client) (*reporter, error) {
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
				r.log.Fatalw("could not parse leases file", "error", err)
			}
			active := ls.FilterActive()
			byMac := active.LatestByMac()
			r.log.Infow("reporting leases to metal-api", "all", len(ls), "active", len(active), "uniqueActive", len(byMac))

			mtx := new(sync.Mutex)
			var items []*leases.ReportItem

			wg := new(sync.WaitGroup)
			wg.Add(len(byMac))

			for _, l := range byMac {
				item := leases.NewReportItem(l, r.log)
				go func() {
					item.EnrichWithBMCDetails(r.cfg.IpmiPort, r.cfg.IpmiUser, r.cfg.IpmiPassword)
					mtx.Lock()
					items = append(items, item)
					wg.Done()
					mtx.Unlock()
				}()
			}

			wg.Wait()

			err = r.report(items)
			if err != nil {
				r.log.Warnw("could not report ipmi addresses", "error", err)
			}
		case <-signals:
			return
		}
	}
}

// report will send all gathered information about machines to the metal-api
func (r reporter) report(items []*leases.ReportItem) error {
	partitionID := r.cfg.PartitionID
	reports := make(map[string]models.V1MachineIpmiReport)

	for _, item := range items {
		mac := item.Mac

		if item.MacContainedIn(r.cfg.IgnoreMacs) {
			continue
		}

		ip := item.Ip
		if item.UUID == nil {
			r.log.Errorw("could not determine uuid of device", "mac", mac, "ip", ip)
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

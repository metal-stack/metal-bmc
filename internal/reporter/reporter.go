package reporter

import (
	"github.com/metal-stack/bmc-catcher/domain"
	"github.com/metal-stack/bmc-catcher/internal/leases"
	metalgo "github.com/metal-stack/metal-go"
	"github.com/metal-stack/metal-go/api/client/machine"
	"github.com/metal-stack/metal-go/api/models"
	"go.uber.org/zap"
)

// Reporter reports information about bmc, bios and dhcp ip of bmc to metal-api
type Reporter struct {
	cfg    *domain.Config
	Log    *zap.SugaredLogger
	client metalgo.Client
}

// NewReporter will create a reporter for MachineIpmiReports
func NewReporter(log *zap.SugaredLogger, cfg *domain.Config, client metalgo.Client) (*Reporter, error) {
	return &Reporter{
		cfg:    cfg,
		Log:    log,
		client: client,
	}, nil
}

// Report will send all gathered information about machines to the metal-api
func (r Reporter) Report(items []*leases.ReportItem) error {
	partitionID := r.cfg.PartitionID
	reports := make(map[string]models.V1MachineIpmiReport)

	for _, item := range items {
		mac := item.Mac

		if item.MacContainedIn(r.cfg.IgnoreMacs) {
			continue
		}

		ip := item.Ip
		if item.UUID == nil {
			r.Log.Errorw("could not determine uuid of device", "mac", mac, "ip", ip)
			continue
		}

		report := models.V1MachineIpmiReport{
			BMCIP:             &item.Ip,
			BMCVersion:        item.BmcVersion,
			BIOSVersion:       item.BiosVersion,
			FRU:               item.FRU,
			PowerState:        item.Powerstate,
			IndicatorLEDState: item.IndicatorLED,
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

	r.Log.Infof("updated ipmi information of %d machines", len(ok.Payload.Updated))
	for _, u := range ok.Payload.Updated {
		r.Log.Infow("ipmi information was updated for machine", "uuid", u)
	}
	for _, u := range ok.Payload.Created {
		r.Log.Infow("ipmi information was set and machine was created", "uuid", u)
	}

	return nil
}

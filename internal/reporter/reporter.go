package reporter

import (
	"github.com/metal-stack/bmc-catcher/domain"
	"github.com/metal-stack/bmc-catcher/internal/leases"
	metalgo "github.com/metal-stack/metal-go"
	"github.com/metal-stack/metal-go/api/models"
	"go.uber.org/zap"
)

// Reporter reports information about bmc, bios and dhcp ip of bmc to metal-api
type Reporter struct {
	cfg    *domain.Config
	Log    *zap.SugaredLogger
	driver *metalgo.Driver
}

// NewReporter will create a reporter for MachineIpmiReports
func NewReporter(cfg *domain.Config, log *zap.SugaredLogger) (*Reporter, error) {
	driver, err := metalgo.NewDriver(cfg.MetalAPIURL.String(), "", cfg.MetalAPIHMACKey, metalgo.AuthType("Metal-Edit"))
	if err != nil {
		return nil, err
	}
	return &Reporter{
		cfg:    cfg,
		Log:    log,
		driver: driver,
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
			BMCIP:       &item.Ip,
			BMCVersion:  item.BmcVersion,
			BIOSVersion: item.BiosVersion,
			FRU:         item.FRU,
		}
		reports[*item.UUID] = report
	}

	mir := metalgo.MachineIPMIReports{
		Reports: &models.V1MachineIpmiReports{
			Partitionid: partitionID,
			Reports:     reports,
		},
	}
	ok, err := r.driver.MachineIPMIReport(mir)
	if err != nil {
		return err
	}

	r.Log.Infof("updated ipmi information of %d machines", len(ok.Response.Updated))
	for _, u := range ok.Response.Updated {
		r.Log.Infow("ipmi information was updated for machine", "uuid", u)
	}
	for _, u := range ok.Response.Created {
		r.Log.Infow("ipmi information was set and machine was created", "uuid", u)
	}

	return nil
}

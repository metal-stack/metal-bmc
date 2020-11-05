package reporter

import (
	"github.com/metal-stack/bmc-catcher/domain"
	"github.com/metal-stack/bmc-catcher/internal/bmc"
	"github.com/metal-stack/bmc-catcher/internal/leases"
	"github.com/metal-stack/go-hal/connect"
	metalgo "github.com/metal-stack/metal-go"
	"github.com/metal-stack/metal-go/api/models"
	"go.uber.org/zap"
)

// Reporter reports information about bmc, bios and dhcp ip of bmc to metal-api
type Reporter struct {
	cfg          *domain.Config
	log          *zap.SugaredLogger
	driver       *metalgo.Driver
	uuidCache    *bmc.UUIDCache
	ipmiPort     int
	ipmiUser     string
	ipmiPassword string
}

// NewReporter will create a reporter for MachineIpmiReports
func NewReporter(cfg *domain.Config, uuidCache *bmc.UUIDCache, log *zap.SugaredLogger, ipmiPort int, ipmiUser, ipmiPassword string) (*Reporter, error) {
	driver, err := metalgo.NewDriver(cfg.MetalAPIURL.String(), "", cfg.MetalAPIHMACKey, metalgo.AuthType("Metal-Edit"))
	if err != nil {
		return nil, err
	}
	return &Reporter{
		cfg:          cfg,
		log:          log,
		driver:       driver,
		uuidCache:    uuidCache,
		ipmiPort:     ipmiPort,
		ipmiUser:     ipmiUser,
		ipmiPassword: ipmiPassword,
	}, nil
}

// Report will send all gathered information about machines to the metal-api
func (r Reporter) Report(ls leases.Leases) error {
	active := ls.FilterActive()
	byMac := active.LatestByMac()
	r.log.Infow("reporting leases to metal-api", "all", len(ls), "active", len(active), "uniqueActive", len(byMac))
	partitionID := r.cfg.PartitionID
	reports := make(map[string]models.V1MachineIPMIReport)

outer:
	for mac, v := range byMac {
		for _, m := range r.cfg.IgnoreMacs {
			if m == mac {
				continue outer
			}
		}

		ip := v.Ip
		uuid, err := r.uuidCache.Get(mac, ip)
		if err != nil {
			r.log.Errorw("could not determine uuid of device", "mac", mac, "ip", ip, "err", err)
			continue
		}

		ob, err := connect.OutBand(v.Ip, r.ipmiPort, r.ipmiUser, r.ipmiPassword)
		if err != nil {
			r.log.Errorw("could not establish outband connection to device bmc", "mac", mac, "ip", ip, "err", err)
			continue
		}

		biosversion := ""
		board := ob.Board()
		if board != nil {
			biosversion = board.BiosVersion
		}
		bmcDetails, err := ob.BMCConnection().BMC()
		if err != nil {
			r.log.Errorw("could not retrieve bmc details of device", "mac", mac, "ip", ip, "err", err)
			continue
		}

		fru := &models.V1MachineFru{
			BoardMfg:            bmcDetails.BoardMfg,
			BoardMfgSerial:      bmcDetails.BoardMfgSerial,
			BoardPartNumber:     bmcDetails.BoardPartNumber,
			ChassisPartNumber:   bmcDetails.ChassisPartNumber,
			ChassisPartSerial:   bmcDetails.ChassisPartSerial,
			ProductManufacturer: bmcDetails.ProductManufacturer,
			ProductPartNumber:   bmcDetails.ProductPartNumber,
			ProductSerial:       bmcDetails.ProductSerial,
		}
		report := models.V1MachineIPMIReport{
			BMCIP:       &ip,
			BMCVersion:  &bmcDetails.FirmwareRevision,
			BIOSVersion: &biosversion,
			FRU:         fru,
		}
		reports[*uuid] = report
	}

	mir := metalgo.MachineIPMIReports{
		Reports: &models.V1MachineIPMIReports{
			Partitionid: partitionID,
			Reports:     reports,
		},
	}
	ok, err := r.driver.MachineIPMIReport(mir)
	if err != nil {
		return err
	}
	r.log.Infof("updated ipmi information of %d machines", len(ok.Response.Updated))
	for _, uuid := range ok.Response.Updated {
		r.log.Infow("ipmi information was updated for machine", "id", uuid)
	}
	for _, uuid := range ok.Response.Created {
		r.log.Infow("ipmi information was set and machine was created", "id", uuid)
	}
	return nil
}

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

type Reporter struct {
	cfg          *domain.Config
	log          *zap.SugaredLogger
	driver       *metalgo.Driver
	uuidCache    *bmc.UUIDCache
	ipmiPort     int
	ipmiUser     string
	ipmiPassword string
}

func NewReporter(cfg *domain.Config, uuidCache *bmc.UUIDCache, log *zap.SugaredLogger, ipmiPort int, ipmiUser, ipmiPassword string) (*Reporter, error) {
	driver, err := metalgo.NewDriver(cfg.MetalAPIURL.String(), "", cfg.MetalAPIHMACKey)
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

func (r Reporter) Report(ls leases.Leases) error {
	active := ls.FilterActive()
	byMac := active.LatestByMac()
	r.log.Infow("reporting leases to metal-api", "all", len(ls), "active", len(active), "uniqueActive", len(byMac))
	partitionID := r.cfg.PartitionID
	l := make(map[string]string)
	f := make(map[string]models.V1MachineFru)
outer:
	for mac, v := range byMac {
		for _, m := range r.cfg.IgnoreMacs {
			if m == mac {
				continue outer
			}
		}
		uuid, err := r.uuidCache.Get(mac, v.Ip)
		if err != nil {
			r.log.Errorw("could not determine uuid of device", "mac", mac, "ip", v.Ip, "err", err)
			continue
		}
		l[*uuid] = v.Ip

		// load FRU information
		ob, err := connect.OutBand(v.Ip, r.ipmiPort, r.ipmiUser, r.ipmiPassword)
		if err != nil {
			r.log.Errorw("could not determine uuid of device", "mac", mac, "ip", v.Ip, "err", err)
		}

		bmc, err := ob.BMCConnection().BMC()
		if err != nil {
			r.log.Errorw("could not determine uuid of device", "mac", mac, "ip", v.Ip, "err", err)
		}

		if bmc != nil {
			f[*uuid] = models.V1MachineFru{
				BoardMfg:            bmc.BoardMfg,
				BoardMfgSerial:      bmc.BoardMfgSerial,
				BoardPartNumber:     bmc.BoardPartNumber,
				ChassisPartNumber:   bmc.ChassisPartNumber,
				ChassisPartSerial:   bmc.ChassisPartSerial,
				ProductManufacturer: bmc.ProductManufacturer,
				ProductPartNumber:   bmc.ProductPartNumber,
				ProductSerial:       bmc.ProductSerial,
			}
		}
	}

	mir := metalgo.MachineIPMIReport{
		Report: &models.V1MachineIPMIReport{
			Partitionid: &partitionID,
			Leases:      l,
			Frus:        f,
		},
	}
	ok, err := r.driver.MachineIPMIReport(mir)
	if err != nil {
		return err
	}
	r.log.Infof("updated ipmi ips of %d machines", len(ok.Response.Updated))
	for uuid, ip := range ok.Response.Updated {
		r.log.Infow("ipmi ip address was updated for machine", "id", uuid, "ip", ip)
	}
	for uuid, ip := range ok.Response.Created {
		r.log.Infow("ipmi ip address was set and machine was created", "id", uuid, "ip", ip)
	}
	return nil
}

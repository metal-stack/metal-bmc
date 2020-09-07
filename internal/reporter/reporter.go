package reporter

import (
	"github.com/metal-stack/bmc-catcher/domain"
	"github.com/metal-stack/bmc-catcher/internal/bmc"
	"github.com/metal-stack/bmc-catcher/internal/leases"
	metalgo "github.com/metal-stack/metal-go"
	"github.com/metal-stack/metal-go/api/models"
	"go.uber.org/zap"
)

type Reporter struct {
	cfg       *domain.Config
	log       *zap.SugaredLogger
	driver    *metalgo.Driver
	uuidCache *bmc.UUIDCache
}

func NewReporter(cfg *domain.Config, uuidCache *bmc.UUIDCache, log *zap.SugaredLogger) (*Reporter, error) {
	driver, err := metalgo.NewDriver(cfg.MetalAPIURL.String(), "", cfg.MetalAPIHMACKey)
	if err != nil {
		return nil, err
	}
	return &Reporter{
		cfg:       cfg,
		log:       log,
		driver:    driver,
		uuidCache: uuidCache,
	}, nil
}

func (r Reporter) Report(ls leases.Leases) error {
	active := ls.FilterActive()
	byMac := active.LatestByMac()
	r.log.Infow("reporting leases to metal-api", "all", len(ls), "active", len(active), "uniqueActive", len(byMac))
	partitionID := r.cfg.PartitionID
	l := map[string]string{}
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
	}
	mir := metalgo.MachineIPMIReport{
		Report: &models.V1MachineIPMIReport{
			Partitionid: &partitionID,
			Leases:      l,
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

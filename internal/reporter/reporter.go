package reporter

import (
	"github.com/metal-stack/ipmi-catcher/domain"
	"github.com/metal-stack/ipmi-catcher/internal/ipmi"
	"github.com/metal-stack/ipmi-catcher/internal/leases"
	"github.com/metal-stack/ipmi-catcher/metal-api/client/machine"
	"github.com/metal-stack/ipmi-catcher/metal-api/models"
	"go.uber.org/zap"
)

type Reporter struct {
	cfg       *domain.Config
	log       *zap.SugaredLogger
	driver    *driver
	mc        *machine.Client
	uuidCache *ipmi.UUIDCache
}

func NewReporter(cfg *domain.Config, uuidCache *ipmi.UUIDCache, log *zap.SugaredLogger) (*Reporter, error) {
	driver, err := newDriver(cfg.MetalAPIURL.String(), cfg.MetalAPIHMACKey)
	mc := driver.machine
	if err != nil {
		return nil, err
	}
	return &Reporter{
		cfg:       cfg,
		log:       log,
		driver:    driver,
		mc:        mc,
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
	params := machine.NewIPMIReportParams()
	req := &models.V1MachineIPMIReport{
		Partitionid: &partitionID,
		Leases:      l,
	}
	params.SetBody(req)
	ok, err := r.mc.IPMIReport(params, r.driver.auth)
	if err != nil {
		return err
	}
	r.log.Infof("updated ipmi ips of %d machines", len(ok.Payload.Updated))
	for uuid, ip := range ok.Payload.Updated {
		r.log.Infow("ipmi ip address was updated for machine", "id", uuid, "ip", ip)
	}
	for uuid, ip := range ok.Payload.Created {
		r.log.Infow("ipmi ip address was set and machine was created", "id", uuid, "ip", ip)
	}
	return nil
}

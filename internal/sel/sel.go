package sel

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/metal-stack/bmc-catcher/domain"
	"github.com/metal-stack/go-hal/connect"
	metalgo "github.com/metal-stack/metal-go"
	"github.com/metal-stack/metal-go/api/models"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type selRunner struct {
	cfg    *domain.Config
	driver *metalgo.Driver
	cache  machineCache
	log    *zap.SugaredLogger
}

type machineCache struct {
	sync.Mutex
	cfg      *domain.Config
	driver   *metalgo.Driver
	machines map[string]*models.V1MachineIPMI
	log      *zap.SugaredLogger
}

func New(cfg *domain.Config, driver *metalgo.Driver, log *zap.SugaredLogger) *selRunner {
	return &selRunner{
		cfg:    cfg,
		driver: driver,
		log:    log,
		cache:  machineCache{driver: driver, log: log, cfg: cfg},
	}
}

func (s *selRunner) Run() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	cacheTicker := time.NewTicker(time.Hour * 24)
	go func() {
		for {
			select {
			case <-cacheTicker.C:
				s.cache.updateCache()
			case <-signals:
				return
			}
		}
	}()

	periodic := time.NewTicker(s.cfg.SelReportInterval)

	for {
		select {
		case <-periodic.C:
			s.fetchSel()
		case <-signals:
			return
		}
	}
}

func (s *selRunner) fetchSel() {

	// FIXME use context to cancel ipmi sel list call
	g, _ := errgroup.WithContext(context.TODO())

	for uuid := range s.cache.machines {
		uuid := uuid
		ipmi := s.cache.machines[uuid]
		if ipmi == nil || ipmi.Address == nil || ipmi.User == nil || ipmi.Password == nil {
			continue
		}
		parts := strings.Split(*ipmi.Address, ":")
		if len(parts) < 2 {
			continue
		}
		ip := parts[0]
		port, err := strconv.Atoi(parts[1])
		if err != nil {
			s.log.Errorw("unable to extract port from ipmi address", "error", err)
			continue
		}
		g.Go(func() error {
			ob, err := connect.OutBand(ip, port, s.cfg.IpmiUser, *ipmi.Password, s.log)
			if err != nil {
				return fmt.Errorf("unable to create ipmi outband connection:%w", err)
			}

			sel := ob.SEL()
			for _, line := range sel {
				s.log.Infow("ipmi sel", "machine", uuid, "log", line)
			}
			return err
		})
	}

	if err := g.Wait(); err != nil {
		s.log.Errorw("unable to fetch sel logs", "error", err)
	}

}

func (m *machineCache) updateCache() {
	m.Lock()
	defer m.Unlock()

	ms, err := m.driver.MachineIPMIList(&metalgo.MachineFindRequest{PartitionID: &m.cfg.PartitionID})
	if err != nil {
		m.log.Errorw("unable to list machine ipmi details", "error", err)
		return
	}

	for _, machine := range ms.Machines {
		ipmi := machine.Ipmi
		if ipmi == nil {
			continue
		}
		m.machines[*machine.ID] = ipmi
	}
}

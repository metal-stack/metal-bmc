package main

import (
	"github.com/metal-stack/go-hal/connect"
	"github.com/metal-stack/metal-go/api/models"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/metal-stack/bmc-catcher/domain"
	"github.com/metal-stack/bmc-catcher/internal/leases"
	"github.com/metal-stack/bmc-catcher/internal/reporter"
	"github.com/metal-stack/v"

	"github.com/kelseyhightower/envconfig"
	halzap "github.com/metal-stack/go-hal/pkg/logger/zap"
	"go.uber.org/zap"
	"go.universe.tf/netboot/dhcp4"
)

func main() {
	logger, _ := zap.NewProduction()
	log := logger.Sugar()
	log.Infow("running app version", "version", v.V.String())
	var cfg domain.Config
	if err := envconfig.Process("BMC_CATCHER", &cfg); err != nil {
		log.Fatalw("bad configuration", "error", err)
	}

	log.Infow("loaded configuration", "config", cfg)

	r, err := reporter.NewReporter(&cfg, log, cfg.IpmiPort, cfg.IpmiUser, cfg.IpmiPassword)
	if err != nil {
		log.Fatalw("could not start reporter", "error", err)
	}

	periodic := time.NewTicker(cfg.ReportInterval)
	dhcpEvents, err := snoopDhcpEvents()
	if err != nil {
		log.Fatalw("could not initialize dhcp snooper", "error", err)
	}
	debounced := time.NewTimer(cfg.DebounceInterval)
	debounced.Stop()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

outer:
	for {
		select {
		case <-periodic.C:
			debounced.Reset(cfg.DebounceInterval)
		case <-dhcpEvents:
			debounced.Reset(cfg.DebounceInterval)
		case <-debounced.C:
			ls, err := leases.ReadLeases(cfg.LeaseFile)
			if err != nil {
				log.Fatalw("could not parse leases file", "error", err)
			}
			active := ls.FilterActive()
			byMac := active.LatestByMac()
			log.Infow("reporting leases to metal-api", "all", len(ls), "active", len(active), "uniqueActive", len(byMac))

			mtx := new(sync.Mutex)
			var items []*leases.ReportItem

			wg := new(sync.WaitGroup)
			wg.Add(len(byMac))

			for mac, l := range byMac {
				mac := mac
				l := l
				go func() {
					defer wg.Done()

					bmcVersion := ""
					biosVersion := ""
					var fru *models.V1MachineFru

					ob, err := connect.OutBand(l.Ip, cfg.IpmiPort, cfg.IpmiUser, cfg.IpmiPassword, halzap.New(r.Log))
					if err != nil {
						log.Errorw("could not establish outband connection to device bmc", "mac", mac, "ip", l.Ip, "err", err)
					} else {
						bmcDetails, err := ob.BMCConnection().BMC()
						if err != nil {
							log.Errorw("could not retrieve bmc details of device", "mac", mac, "ip", l.Ip, "err", err)
						} else {
							bmcVersion = bmcDetails.FirmwareRevision
							fru = &models.V1MachineFru{
								BoardMfg:            bmcDetails.BoardMfg,
								BoardMfgSerial:      bmcDetails.BoardMfgSerial,
								BoardPartNumber:     bmcDetails.BoardPartNumber,
								ChassisPartNumber:   bmcDetails.ChassisPartNumber,
								ChassisPartSerial:   bmcDetails.ChassisPartSerial,
								ProductManufacturer: bmcDetails.ProductManufacturer,
								ProductPartNumber:   bmcDetails.ProductPartNumber,
								ProductSerial:       bmcDetails.ProductSerial,
							}
						}

						board := ob.Board()
						if board != nil {
							biosVersion = board.BiosVersion
						}
					}

					item := &leases.ReportItem{
						Lease:       l,
						FRU:         fru,
						BmcVersion:  &bmcVersion,
						BiosVersion: &biosVersion,
					}

					mtx.Lock()
					items = append(items, item)
					mtx.Unlock()
				}()
			}

			wg.Wait()

			err = r.Report(items)
			if err != nil {
				log.Warnw("could not report ipmi addresses", "error", err)
			}
		case <-signals:
			break outer
		}
	}
}

func snoopDhcpEvents() (chan dhcp4.Packet, error) {
	c := make(chan dhcp4.Packet, 10)
	dhcp, err := dhcp4.NewSnooperConn("0.0.0.0:67")
	if err != nil {
		return nil, err
	}
	go func() {
		for {
			p, _, err := dhcp.RecvDHCP()
			if err == nil {
				c <- *p
			}
		}
	}()
	return c, nil
}

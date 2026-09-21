package leases

import (
	"log/slog"
	"time"

	apiv2 "github.com/metal-stack/api/go/metalstack/api/v2"
	"github.com/metal-stack/go-hal"
	"github.com/metal-stack/go-hal/connect"
	halslog "github.com/metal-stack/go-hal/pkg/logger/slog"
)

func (i *ReportItem) EnrichWithBMCDetails(log *slog.Logger, ipmiPort int, ipmiUser, ipmiPassword string) error {
	log = log.With("mac", i.Lease.Mac, "ip", i.Lease.Ip)

	ob, err := connect.OutBand(i.Lease.Ip, ipmiPort, ipmiUser, ipmiPassword, halslog.New(log), new(time.Minute))
	if err != nil {
		log.Error("could not establish outband connection to device bmc", "err", err)
		return err
	}

	bmcDetails, err := ob.BMCConnection().BMC()
	if err != nil {
		log.Warn("could not retrieve bmc details of device", "err", err)
		return err
	}

	u, err := ob.UUID()
	if err != nil {
		log.Warn("could not determine uuid of device", "err", err)
		return err
	}

	i.MachineBMCReport = apiv2.MachineBMCReport{
		Uuid: u.String(),
		Bmc: &apiv2.MachineBMC{
			Address: i.Lease.Ip,
			Mac:     i.Lease.Mac,
			Version: bmcDetails.FirmwareRevision,
		},
		Fru: &apiv2.MachineFRU{
			ChassisPartNumber:   new(bmcDetails.ChassisPartNumber),
			ChassisPartSerial:   new(bmcDetails.ChassisPartSerial),
			BoardMfg:            new(bmcDetails.BoardMfg),
			BoardMfgSerial:      new(bmcDetails.BoardMfgSerial),
			BoardPartNumber:     new(bmcDetails.BoardPartNumber),
			ProductManufacturer: new(bmcDetails.ProductManufacturer),
			ProductPartNumber:   new(bmcDetails.ProductPartNumber),
			ProductSerial:       new(bmcDetails.ProductSerial),
		},
	}

	powerState, err := ob.PowerState()
	if err == nil {
		i.Bmc.PowerState = powerState.String()
	} else {
		log.Warn("could not retrieve power state", "err", err)
		i.Bmc.PowerState = hal.PowerUnknownState.String()
	}

	board := ob.Board()

	if board != nil {
		i.Bios = &apiv2.MachineBios{
			Version: board.BiosVersion,
		}

		i.LedState = &apiv2.MachineChassisIdentifyLEDState{
			Value: board.IndicatorLED,
		}

		if board.PowerMetric != nil {
			i.PowerMetric = &apiv2.MachinePowerMetric{
				AverageConsumedWatts: board.PowerMetric.AverageConsumedWatts,
				IntervalInMin:        board.PowerMetric.IntervalInMin,
				MaxConsumedWatts:     board.PowerMetric.MaxConsumedWatts,
				MinConsumedWatts:     board.PowerMetric.MinConsumedWatts,
			}
		}

		i.PowerSupplies = func() []*apiv2.MachinePowerSupply {
			var res []*apiv2.MachinePowerSupply

			for _, ps := range board.PowerSupplies {
				res = append(res, &apiv2.MachinePowerSupply{
					Health: ps.Status.Health,
					State:  ps.Status.State,
				})
			}

			return res
		}()
	}

	return nil
}

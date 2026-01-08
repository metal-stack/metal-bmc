package leases

import (
	apiv2 "github.com/metal-stack/api/go/metalstack/api/v2"
	"github.com/metal-stack/go-hal"
	"github.com/metal-stack/go-hal/connect"
	halslog "github.com/metal-stack/go-hal/pkg/logger/slog"
)

func (i *ReportItem) EnrichWithBMCDetails(ipmiPort int, ipmiUser, ipmiPassword string) error {
	ob, err := connect.OutBand(i.Lease.Ip, ipmiPort, ipmiUser, ipmiPassword, halslog.New(i.Log))
	if err != nil {
		i.Log.Error("could not establish outband connection to device bmc", "mac", i.Lease.Mac, "ip", i.Lease.Ip, "err", err)
		return err
	}

	bmcDetails, err := ob.BMCConnection().BMC()
	if err == nil {
		i.BmcVersion = &bmcDetails.FirmwareRevision
		i.FRU = &apiv2.MachineFRU{
			BoardMfg:            &bmcDetails.BoardMfg,
			BoardMfgSerial:      &bmcDetails.BoardMfgSerial,
			BoardPartNumber:     &bmcDetails.BoardPartNumber,
			ChassisPartNumber:   &bmcDetails.ChassisPartNumber,
			ChassisPartSerial:   &bmcDetails.ChassisPartSerial,
			ProductManufacturer: &bmcDetails.ProductManufacturer,
			ProductPartNumber:   &bmcDetails.ProductPartNumber,
			ProductSerial:       &bmcDetails.ProductSerial,
		}
	} else {
		i.Log.Warn("could not retrieve bmc details of device", "mac", i.Lease.Mac, "ip", i.Lease.Ip, "err", err)
		return err
	}

	powerState, err := ob.PowerState()
	state := hal.PowerUnknownState.String()
	if err == nil {
		state = powerState.String()
	}
	i.Powerstate = &state

	board := ob.Board()
	if board != nil {
		i.BiosVersion = &board.BiosVersion
		i.IndicatorLED = &board.IndicatorLED
		if board.PowerMetric != nil {
			i.PowerMetric = &apiv2.MachinePowerMetric{
				AverageConsumedWatts: board.PowerMetric.AverageConsumedWatts,
				IntervalInMin:        board.PowerMetric.IntervalInMin,
				MaxConsumedWatts:     board.PowerMetric.MaxConsumedWatts,
				MinConsumedWatts:     board.PowerMetric.MinConsumedWatts,
			}
		}
		var powerSupplies []*apiv2.MachinePowerSupply
		for _, ps := range board.PowerSupplies {
			powerSupplies = append(powerSupplies, &apiv2.MachinePowerSupply{
				Health: ps.Status.Health,
				State:  ps.Status.State,
			})
		}
		i.PowerSupplies = powerSupplies
	}

	u, err := ob.UUID()
	if err == nil {
		str := u.String()
		i.UUID = &str
	} else {
		i.Log.Warn("could not determine uuid of device", "mac", i.Lease.Mac, "ip", i.Lease.Ip, "err", err)
		return err
	}
	return nil
}

package leases

import (
	"github.com/metal-stack/go-hal"
	"github.com/metal-stack/go-hal/connect"
	halslog "github.com/metal-stack/go-hal/pkg/logger/slog"
	"github.com/metal-stack/metal-go/api/models"
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
		i.FRU = &models.V1MachineFru{
			BoardMfg:            bmcDetails.BoardMfg,
			BoardMfgSerial:      bmcDetails.BoardMfgSerial,
			BoardPartNumber:     bmcDetails.BoardPartNumber,
			ChassisPartNumber:   bmcDetails.ChassisPartNumber,
			ChassisPartSerial:   bmcDetails.ChassisPartSerial,
			ProductManufacturer: bmcDetails.ProductManufacturer,
			ProductPartNumber:   bmcDetails.ProductPartNumber,
			ProductSerial:       bmcDetails.ProductSerial,
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
			i.PowerMetric = &models.V1PowerMetric{
				Averageconsumedwatts: &board.PowerMetric.AverageConsumedWatts,
				Intervalinmin:        &board.PowerMetric.IntervalInMin,
				Maxconsumedwatts:     &board.PowerMetric.MaxConsumedWatts,
				Minconsumedwatts:     &board.PowerMetric.MinConsumedWatts,
			}
		}
		var powerSupplies []*models.V1PowerSupply
		for _, ps := range board.PowerSupplies {
			powerSupplies = append(powerSupplies, &models.V1PowerSupply{
				Status: &models.V1PowerSupplyStatus{
					Health: &ps.Status.Health,
					State:  &ps.Status.State,
				},
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

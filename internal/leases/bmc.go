package leases

import (
	"github.com/metal-stack/go-hal"
	"github.com/metal-stack/go-hal/connect"
	halslog "github.com/metal-stack/go-hal/pkg/logger/slog"
	"github.com/metal-stack/metal-go/api/models"
)

func (i *ReportItem) EnrichWithBMCDetails(ipmiPort int, ipmiUser, ipmiPassword string) {
	ob, err := connect.OutBand(i.Ip, ipmiPort, ipmiUser, ipmiPassword, halslog.New(i.Log))
	if err != nil {
		i.Log.Error("could not establish outband connection to device bmc", "mac", i.Mac, "ip", i.Ip, "err", err)
		return
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
		i.Log.Warn("could not retrieve bmc details of device", "mac", i.Mac, "ip", i.Ip, "err", err)
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
	}

	u, err := ob.UUID()
	if err == nil {
		str := u.String()
		i.UUID = &str
	} else {
		i.Log.Warn("could not determine uuid of device", "mac", i.Mac, "ip", i.Ip, "err", err)
	}
}

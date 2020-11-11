package leases

import (
	"github.com/metal-stack/go-hal/connect"
	halzap "github.com/metal-stack/go-hal/pkg/logger/zap"
	"github.com/metal-stack/metal-go/api/models"
)

func (i *ReportItem) EnrichWithBMCDetails() {
	ip := i.Ip
	ob, err := connect.OutBand(ip, i.Config.IpmiPort, i.Config.IpmiUser, i.Config.IpmiPassword, halzap.New(i.Log))
	if err != nil {
		i.Log.Errorw("could not establish outband connection to device bmc", "mac", i.Mac, "ip", ip, "err", err)
	} else {
		bmcDetails, err := ob.BMCConnection().BMC()
		if err != nil {
			i.Log.Errorw("could not retrieve bmc details of device", "mac", i.Mac, "ip", ip, "err", err)
		} else {
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
		}

		board := ob.Board()
		if board != nil {
			i.BiosVersion = &board.BiosVersion
		}
	}
}

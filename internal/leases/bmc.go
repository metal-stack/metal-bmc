package leases

import (
	"github.com/metal-stack/go-hal/connect"
	halzap "github.com/metal-stack/go-hal/pkg/logger/zap"
	"github.com/metal-stack/metal-go/api/models"
)

func (i *ReportItem) EnrichWithBMCDetails(ipmiPort int, ipmiUser, ipmiPassword string) {
	ob, err := connect.OutBand(i.Ip, ipmiPort, ipmiUser, ipmiPassword, halzap.New(i.Log))
	if err != nil {
		i.Log.Errorw("could not establish outband connection to device bmc", "mac", i.Mac, "ip", i.Ip, "err", err)
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
		i.Log.Errorw("could not retrieve bmc details of device", "mac", i.Mac, "ip", i.Ip, "err", err)
	}

	board := ob.Board()
	if board != nil {
		i.BiosVersion = &board.BiosVersion
	}

	u, err := ob.UUID()
	if err == nil {
		str := u.String()
		i.UUID = &str
	} else {
		i.Log.Errorw("could not determine uuid of device", "mac", i.Mac, "ip", i.Ip, "err", err)
	}
}

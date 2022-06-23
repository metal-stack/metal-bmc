package bmc

import (
	"fmt"

	"github.com/metal-stack/go-hal"
	metalgo "github.com/metal-stack/metal-go"
)

func (b *BMCService) UpdateFirmware(outBand hal.OutBand, event *MachineEvent) error {
	if event.Cmd.FirmwareUpdate == nil {
		return fmt.Errorf("firmwareupdate is nil")
	}

	fw := event.Cmd.FirmwareUpdate
	switch fw.Kind {
	case string(metalgo.Bios):
		go func() {
			err := outBand.UpdateBIOS(fw.URL)
			if err != nil {
				b.log.Errorw("updatebios", "error", err)
			}
		}()
	case string(metalgo.Bmc):
		go func() {
			err := outBand.UpdateBMC(fw.URL)
			if err != nil {
				b.log.Errorw("updatebmc", "error", err)
			}
		}()
	default:
		return fmt.Errorf("unknown firmware kind %q", fw.Kind)
	}
	return nil
}

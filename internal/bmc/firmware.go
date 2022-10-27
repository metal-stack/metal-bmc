package bmc

import (
	"fmt"

	"github.com/metal-stack/go-hal"
	"github.com/metal-stack/metal-go/api/models"
)

func (b *BMCService) UpdateFirmware(outBand hal.OutBand, event *MachineEvent) error {
	b.log.Infow("update firmware", "event", event)
	if event.Cmd.FirmwareUpdate == nil {
		return fmt.Errorf("firmwareupdate is nil")
	}

	fw := event.Cmd.FirmwareUpdate
	switch fw.Kind {
	case models.V1MachineUpdateFirmwareRequestKindBios:
		b.log.Infow("update firmware bios", "download url", fw.URL)
		go func() {
			err := outBand.UpdateBIOS(fw.URL)
			if err != nil {
				b.log.Errorw("updatebios", "error", err)
			}
		}()
	case models.V1MachineUpdateFirmwareRequestKindBmc:
		b.log.Infow("update firmware bmc", "download url", fw.URL)
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

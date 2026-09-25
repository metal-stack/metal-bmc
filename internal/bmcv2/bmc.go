package bmcv2

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	apiclient "github.com/metal-stack/api/go/client"
	apiv2 "github.com/metal-stack/api/go/metalstack/api/v2"
	infrav2 "github.com/metal-stack/api/go/metalstack/infra/v2"
	"github.com/metal-stack/go-hal"
	halconnect "github.com/metal-stack/go-hal/connect"
	halslog "github.com/metal-stack/go-hal/pkg/logger/slog"
	"github.com/metal-stack/metal-bmc/pkg/config"
)

type V2 struct {
	log    *slog.Logger
	cfg    *config.Config
	client apiclient.Client
}

func New(log *slog.Logger, client apiclient.Client, c *config.Config) *V2 {
	return &V2{
		log:    log,
		cfg:    c,
		client: client,
	}
}

const reconnectDelay = 5 * time.Second

func (b *V2) ProcessCommands(ctx context.Context) {
	msgs, errs := apiclient.ReconnectingStreamRead(ctx, func(ctx context.Context) (*connect.ServerStreamForClient[infrav2.WaitForBMCCommandResponse], error) {
		return b.client.Infrav2().BMC().WaitForBMCCommand(ctx, &infrav2.WaitForBMCCommandRequest{Partition: b.cfg.PartitionID})
	}, apiclient.WithStreamBackoff(reconnectDelay), apiclient.WithStreamLogger(b.log))

	for {
		select {
		case message := <-msgs:
			log := b.log.With("machine", message.Uuid, "command", message.BmcCommand.String(), "bmc", message.MachineBmc)

			log.Info("handle v2 command")

			err := b.handleMessage(ctx, log, message)
			if err != nil {
				log.Error("error handling v2 command", "error", err)
			} else {
				log.Info("successfully handled v2 command")
			}

		case err := <-errs:
			b.log.Error("error handling v2 command", "error", err)

		case <-ctx.Done():
			b.log.Info("context cancelled, stop processing v2 commands")
			return
		}
	}
}

func (b *V2) handleMessage(ctx context.Context, log *slog.Logger, message *infrav2.WaitForBMCCommandResponse) error {
	var (
		bmcCommandFunc func() error
		req            = &infrav2.BMCCommandDoneRequest{CommandId: message.CommandId}
	)

	defer func() {
		_, err := b.client.Infrav2().BMC().BMCCommandDone(ctx, req)
		if err != nil {
			log.Error("error sending bmc command done response", "error", err)
		}
	}()

	if message.MachineBmc == nil {
		err := fmt.Errorf("event does not contain bmc details: %v", message)
		req.Error = new(err.Error())
		return err
	}

	outBand, err := b.outBand(message.MachineBmc)
	if err != nil {
		log.Error("error creating outband connection", "error", err)
		req.Error = new(err.Error())
		return err
	}

	switch message.BmcCommand {
	case apiv2.MachineBMCCommand_MACHINE_BMC_COMMAND_ON:
		bmcCommandFunc = outBand.PowerOn
	case apiv2.MachineBMCCommand_MACHINE_BMC_COMMAND_OFF:
		bmcCommandFunc = outBand.PowerOff
	case apiv2.MachineBMCCommand_MACHINE_BMC_COMMAND_RESET:
		bmcCommandFunc = outBand.PowerReset
	case apiv2.MachineBMCCommand_MACHINE_BMC_COMMAND_CYCLE:
		bmcCommandFunc = outBand.PowerCycle
	case apiv2.MachineBMCCommand_MACHINE_BMC_COMMAND_BOOT_TO_BIOS:
		bmcCommandFunc = func() error { return outBand.BootFrom(hal.BootTargetBIOS) }
	case apiv2.MachineBMCCommand_MACHINE_BMC_COMMAND_BOOT_FROM_DISK:
		bmcCommandFunc = func() error { return outBand.BootFrom(hal.BootTargetDisk) }
	case apiv2.MachineBMCCommand_MACHINE_BMC_COMMAND_BOOT_FROM_PXE:
		bmcCommandFunc = func() error { return outBand.BootFrom(hal.BootTargetPXE) }
	case apiv2.MachineBMCCommand_MACHINE_BMC_COMMAND_IDENTIFY_LED_ON:
		bmcCommandFunc = outBand.IdentifyLEDOn
	case apiv2.MachineBMCCommand_MACHINE_BMC_COMMAND_IDENTIFY_LED_OFF:
		bmcCommandFunc = outBand.IdentifyLEDOff
	case apiv2.MachineBMCCommand_MACHINE_BMC_COMMAND_MACHINE_CREATED:
		bmcCommandFunc = func() error {
			return outBand.BootFrom(hal.BootTargetDisk)
		}
	case apiv2.MachineBMCCommand_MACHINE_BMC_COMMAND_MACHINE_DELETED:
		bmcCommandFunc = func() error {
			err := outBand.BootFrom(hal.BootTargetPXE)
			if err != nil {
				return err
			}
			return outBand.PowerReset()
		}
	default:
		bmcCommandFunc = func() error {
			return fmt.Errorf("unsupported bmc command was issued: %s", message.BmcCommand.String())
		}
	}

	if err := bmcCommandFunc(); err != nil {
		req.Error = new(err.Error())
	}

	return nil
}

func (b *V2) outBand(bmc *apiv2.MachineBMC) (hal.OutBand, error) {
	host, portString, found := strings.Cut(bmc.Address, ":")
	if !found {
		portString = "623"
	}

	port, err := strconv.Atoi(portString)
	if err != nil {
		return nil, fmt.Errorf("unable to convert port to an int: %w", err)
	}

	outBand, err := halconnect.OutBand(host, port, bmc.User, bmc.Password, halslog.New(b.log), nil)
	if err != nil {
		return nil, err
	}

	return outBand, nil
}

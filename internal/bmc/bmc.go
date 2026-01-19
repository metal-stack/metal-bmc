package bmc

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"

	apiclient "github.com/metal-stack/api/go/client"
	"github.com/metal-stack/api/go/enum"
	apiv2 "github.com/metal-stack/api/go/metalstack/api/v2"
	infrav2 "github.com/metal-stack/api/go/metalstack/infra/v2"
	"github.com/metal-stack/go-hal"
	"github.com/metal-stack/go-hal/connect"
	halslog "github.com/metal-stack/go-hal/pkg/logger/slog"
	"github.com/metal-stack/metal-bmc/pkg/config"
)

type BMCService struct {
	log    *slog.Logger
	cfg    *config.Config
	client apiclient.Client
}

func New(log *slog.Logger, client apiclient.Client, c *config.Config) *BMCService {
	b := &BMCService{
		log:    log,
		cfg:    c,
		client: client,
	}
	return b
}

func (b *BMCService) ProcessCommands() {
	b.log.Info("processCommand, start waiting for bmc commands")

Retry:

	messageChan, errChan := b.subscribeAsync(context.Background(), b.cfg.PartitionID)
	select {
	case message := <-messageChan:
		err := b.handleMessage(message)
		if err != nil {
			b.log.Error("processCommand", "error", err)
		}
	case err := <-errChan:
		if err == io.EOF {
			b.log.Error("processCommand stream ended", "error", err)
		}
		if err == context.Canceled {
			b.log.Error("processCommand context canceled", "error", err)
		}
		b.log.Error("processCommand", "error", err)
		goto Retry
	}

}

func (b *BMCService) handleMessage(message *infrav2.WaitForBMCCommandResponse) error {

	var command string
	commandString, err := enum.GetStringValue(message.BmcCommand)
	if err != nil {
		command = message.BmcCommand.String()
	} else {
		command = *commandString
	}

	b.log.Info("handlemessage", "machine", message.Uuid, "command", command, "bmc details", message.MachineBmc)

	if message.MachineBmc == nil {
		return fmt.Errorf("event does not contain bmc details:%v", message)
	}
	outBand, err := b.outBand(message.MachineBmc)
	if err != nil {
		b.log.Error("error creating outband connection", "error", err)
		return err
	}

	switch message.BmcCommand {
	case apiv2.MachineBMCCommand_MACHINE_BMC_COMMAND_MACHINE_DELETED:
		err := outBand.BootFrom(hal.BootTargetPXE)
		if err != nil {
			return err
		}
		return outBand.PowerReset()
	case apiv2.MachineBMCCommand_MACHINE_BMC_COMMAND_ON:
		return outBand.PowerOn()
	case apiv2.MachineBMCCommand_MACHINE_BMC_COMMAND_OFF:
		return outBand.PowerOff()
	case apiv2.MachineBMCCommand_MACHINE_BMC_COMMAND_RESET:
		return outBand.PowerReset()
	case apiv2.MachineBMCCommand_MACHINE_BMC_COMMAND_CYCLE:
		return outBand.PowerCycle()
	case apiv2.MachineBMCCommand_MACHINE_BMC_COMMAND_BOOT_TO_BIOS:
		return outBand.BootFrom(hal.BootTargetBIOS)
	case apiv2.MachineBMCCommand_MACHINE_BMC_COMMAND_BOOT_FROM_DISK:
		return outBand.BootFrom(hal.BootTargetDisk)
	case apiv2.MachineBMCCommand_MACHINE_BMC_COMMAND_BOOT_FROM_PXE:
		return outBand.BootFrom(hal.BootTargetPXE)
	case apiv2.MachineBMCCommand_MACHINE_BMC_COMMAND_IDENTIFY_LED_ON:
		return outBand.IdentifyLEDOn()
	case apiv2.MachineBMCCommand_MACHINE_BMC_COMMAND_IDENTIFY_LED_OFF:
		return outBand.IdentifyLEDOff()
	case apiv2.MachineBMCCommand_MACHINE_BMC_COMMAND_MACHINE_CREATED:
		return outBand.BootFrom(hal.BootTargetDisk)
	default:
		b.log.Warn("unhandled command", "command", message.BmcCommand.String())
	}
	return nil
}

// messageHandler is called when a message is received
type messageHandler func(*infrav2.WaitForBMCCommandResponse) error

// Subscribe subscribes to a topic and calls the handler for each message
func (c *BMCService) subscribe(ctx context.Context, topic string, handler messageHandler) error {
	stream, err := c.client.Infrav2().BMC().WaitForBMCCommand(ctx, &infrav2.WaitForBMCCommandRequest{Partition: topic})
	if err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}
	defer func() {
		_ = stream.Close()
	}()

	c.log.Info("subscribed to machine bmc command", "topic", topic)

	// Receive messages
	for stream.Receive() {
		msg := stream.Msg()
		if err := handler(msg); err != nil {
			c.log.Error("handler error", "error", err)
		}
	}

	if err := stream.Err(); err != nil {
		if err == io.EOF || err == context.Canceled {
			return nil
		}
		return fmt.Errorf("stream error: %w", err)
	}

	return nil
}

// SubscribeAsync subscribes asynchronously and returns a channel of messages
func (c *BMCService) subscribeAsync(ctx context.Context, topic string) (<-chan *infrav2.WaitForBMCCommandResponse, <-chan error) {
	var (
		msgChan = make(chan *infrav2.WaitForBMCCommandResponse, 100)
		errChan = make(chan error, 1)
	)

	go func() {
		defer close(msgChan)
		defer close(errChan)

		err := c.subscribe(ctx, topic, func(msg *infrav2.WaitForBMCCommandResponse) error {
			select {
			case msgChan <- msg:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})

		if err != nil && err != context.Canceled {
			errChan <- err
		}
	}()

	return msgChan, errChan
}

func (b *BMCService) outBand(bmc *apiv2.MachineBMC) (hal.OutBand, error) {
	host, portString, found := strings.Cut(bmc.Address, ":")
	if !found {
		portString = "623"

	}
	port, err := strconv.Atoi(portString)
	if err != nil {
		return nil, fmt.Errorf("unable to convert port to an int %w", err)
	}
	outBand, err := connect.OutBand(host, port, bmc.User, bmc.Password, halslog.New(b.log))
	if err != nil {
		return nil, err
	}
	return outBand, nil
}

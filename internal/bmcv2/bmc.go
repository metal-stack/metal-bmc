package bmcv2

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"time"

	apiclient "github.com/metal-stack/api/go/client"
	apiv2 "github.com/metal-stack/api/go/metalstack/api/v2"
	infrav2 "github.com/metal-stack/api/go/metalstack/infra/v2"
	"github.com/metal-stack/go-hal"
	"github.com/metal-stack/go-hal/connect"
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
	for {
		err := b.processCommandStream(ctx)
		if ctx.Err() != nil {
			b.log.Info("received stop signal, stop serving bmc commands")
			return
		}

		b.log.Error("command stream ended, reconnecting", "error", err)

		select {
		case <-time.After(reconnectDelay):
		case <-ctx.Done():
			b.log.Info("received stop signal, stop serving bmc commands")
			return
		}
	}
}

func (b *V2) processCommandStream(ctx context.Context) error {
	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	messageChan, errChan := b.subscribeAsync(streamCtx, b.cfg.PartitionID)

	b.log.Info("subscribed to receiving v2 bmc commands")

	for {
		select {
		case message, ok := <-messageChan:
			if !ok {
				return io.EOF
			}
			if message == nil {
				continue
			}

			log := b.log.With("machine", message.Uuid, "command", message.BmcCommand.String(), "bmc", message.MachineBmc)

			log.Info("handle v2 command")

			err := b.handleMessage(ctx, log, message)
			if err != nil {
				log.Error("error handling v2 command", "error", err)
			} else {
				log.Info("successfully handled v2 command")
			}
		case err, ok := <-errChan:
			if !ok || err == nil {
				return io.EOF
			}
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (b *V2) handleMessage(ctx context.Context, log *slog.Logger, message *infrav2.WaitForBMCCommandResponse) error {
	if message.MachineBmc == nil {
		return fmt.Errorf("event does not contain bmc details: %v", message)
	}

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

// messageHandler is called when a message is received
type messageHandler func(*infrav2.WaitForBMCCommandResponse) error

// Subscribe subscribes to a topic and calls the handler for each message
func (c *V2) subscribe(ctx context.Context, topic string, handler messageHandler) error {
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
		c.log.Info("machine bmc message received", "message", msg)
		if err := handler(msg); err != nil {
			if ctx.Err() != nil {
				return nil
			}
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

// subscribeAsync subscribes asynchronously and returns a channel of messages
func (c *V2) subscribeAsync(ctx context.Context, topic string) (<-chan *infrav2.WaitForBMCCommandResponse, <-chan error) {
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

func (b *V2) outBand(bmc *apiv2.MachineBMC) (hal.OutBand, error) {
	host, portString, found := strings.Cut(bmc.Address, ":")
	if !found {
		portString = "623"
	}

	port, err := strconv.Atoi(portString)
	if err != nil {
		return nil, fmt.Errorf("unable to convert port to an int: %w", err)
	}

	outBand, err := connect.OutBand(host, port, bmc.User, bmc.Password, halslog.New(b.log), nil)
	if err != nil {
		return nil, err
	}

	return outBand, nil
}

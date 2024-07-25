package bmc

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/metal-stack/go-hal"
	"github.com/nsqio/go-nsq"
)

const (
	mqChannel = "core"
)

func (b *BMCService) InitConsumer() error {
	caCertRaw, err := os.ReadFile(b.mqCACertFile)
	if err != nil {
		return fmt.Errorf("failed to read ca cert: %w", err)
	}

	caCertPool, err := x509.SystemCertPool()
	if err != nil {
		return err
	}

	ok := caCertPool.AppendCertsFromPEM(caCertRaw)
	if !ok {
		return fmt.Errorf("unable to add ca to cert pool")
	}

	cert, err := tls.LoadX509KeyPair(b.mqClientCertFile, b.mqClientCertKeyFile)
	if err != nil {
		return err
	}

	config := nsq.NewConfig()
	config.TlsConfig = &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientCAs:    caCertPool,
		RootCAs:      caCertPool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS12,
	}
	config.TlsV1 = true

	// Deadlines for network reads and writes
	config.ReadTimeout = 10 * time.Second
	config.WriteTimeout = 10 * time.Second

	// Duration of time between heartbeats. This must be less than ReadTimeout
	config.HeartbeatInterval = 5 * time.Second

	// Maximum duration when REQueueing (for doubling of deferred requeue)
	config.MaxRequeueDelay = 10 * time.Second
	config.DefaultRequeueDelay = 5 * time.Second

	// Maximum amount of time to backoff when processing fails 0 == no backoff
	config.MaxBackoffDuration = 0 * time.Second // no need for backing off, just requeue

	// Maximum number of times this consumer will attempt to process a message before giving up
	config.MaxAttempts = 2 // we do not try very often, if it doesn't work it's probably for a reason

	// Maximum number of messages to allow in flight (concurrency knob)
	config.MaxInFlight = 10 // handling 10 machines in parallel should be enough

	consumer, err := nsq.NewConsumer(b.machineTopic, mqChannel, config)
	if err != nil {
		return err
	}

	consumer.SetLogger(nsqLogger{log: b.log}, nsqMapLevel(b.log))
	consumer.AddHandler(b)

	err = consumer.ConnectToNSQD(b.mqAddress)
	if err != nil {
		return err
	}

	return err
}

func (b *BMCService) HandleMessage(message *nsq.Message) error {
	var event MachineEvent
	err := json.Unmarshal(message.Body, &event)
	if err != nil {
		return err
	}

	b.log.Info("got message from nsq", "message-id", message.ID, "topic", b.machineTopic, "event", event, "attempt", message.Attempts)

	if event.Cmd.IPMI == nil {
		return fmt.Errorf("event does not contain ipmi details:%v", event)
	}
	outBand, err := b.outBand(event.Cmd.IPMI)
	if err != nil {
		b.log.Error("error creating outband connection", "error", err)
		return err
	}

	switch event.Type {
	case Delete:
		err := outBand.BootFrom(hal.BootTargetPXE)
		if err != nil {
			return err
		}
		return outBand.PowerReset()
	case Command:
		switch event.Cmd.Command {
		case MachineOnCmd:
			return outBand.PowerOn()
		case MachineOffCmd:
			return outBand.PowerOff()
		case MachineResetCmd:
			return outBand.PowerReset()
		case MachineCycleCmd:
			return outBand.PowerCycle()
		case MachineBiosCmd:
			return outBand.BootFrom(hal.BootTargetBIOS)
		case MachineDiskCmd:
			return outBand.BootFrom(hal.BootTargetDisk)
		case MachinePxeCmd:
			return outBand.BootFrom(hal.BootTargetPXE)
		case MachineReinstallCmd:
			err := outBand.BootFrom(hal.BootTargetPXE)
			if err != nil {
				return err
			}
			return outBand.PowerCycle()
		case ChassisIdentifyLEDOnCmd:
			return outBand.IdentifyLEDOn()
		case ChassisIdentifyLEDOffCmd:
			return outBand.IdentifyLEDOff()
		case UpdateFirmwareCmd:
			return b.UpdateFirmware(outBand, &event)
		default:
			b.log.Error("unhandled command", "topic", b.machineTopic, "channel", "core", "event", event)
		}
	case Create, Update:
		fallthrough
	default:
		b.log.Warn("unhandled event", "topic", b.machineTopic, "channel", "core", "event", event)
	}
	return nil
}

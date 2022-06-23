package bmc

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/metal-stack/go-hal"
	"github.com/metal-stack/go-hal/connect"
	halzap "github.com/metal-stack/go-hal/pkg/logger/zap"

	"go.uber.org/zap"
)

type BMCService struct {
	log *zap.SugaredLogger
	// NSQ related config options
	mqAddress        string
	mqCACertFile     string
	mqClientCertFile string
	mqLogLevel       string
	machineTopic     string
	machineTopicTTL  time.Duration
}

type Config struct {
	Log              *zap.SugaredLogger
	MQAddress        string
	MQCACertFile     string
	MQClientCertFile string
	MQLogLevel       string
	MachineTopic     string
	MachineTopicTTL  time.Duration
}

func New(c Config) *BMCService {
	b := &BMCService{
		log:              c.Log,
		mqAddress:        c.MQAddress,
		mqCACertFile:     c.MQCACertFile,
		mqClientCertFile: c.MQClientCertFile,
		mqLogLevel:       c.MQLogLevel,
		machineTopic:     c.MachineTopic,
		machineTopicTTL:  c.MachineTopicTTL,
	}
	return b
}

type MachineEvent struct {
	Type         EventType           `json:"type,omitempty"`
	OldMachineID string              `json:"old,omitempty"`
	Cmd          *MachineExecCommand `json:"cmd,omitempty"`
}

type MachineExecCommand struct {
	TargetMachineID string          `json:"target,omitempty"`
	Command         MachineCommand  `json:"cmd,omitempty"`
	IPMI            *IPMI           `json:"ipmi,omitempty"`
	FirmwareUpdate  *FirmwareUpdate `json:"firmwareupdate,omitempty"`
}

type IPMI struct {
	// Address is host:port of the connection to the ipmi BMC, host can be either a ip address or a hostname
	Address  string `json:"address"`
	User     string `json:"user"`
	Password string `json:"password"`
	Fru      Fru    `json:"fru"`
}

type FirmwareUpdate struct {
	Kind string `json:"kind"`
	URL  string `json:"url"`
}

type Fru struct {
	BoardPartNumber string `json:"board_part_number"`
}

type MachineCommand string

// FIXME these constants must move to a single location
const (
	MachineOnCmd             MachineCommand = "ON"
	MachineOffCmd            MachineCommand = "OFF"
	MachineResetCmd          MachineCommand = "RESET"
	MachineCycleCmd          MachineCommand = "CYCLE"
	MachineBiosCmd           MachineCommand = "BIOS"
	MachineDiskCmd           MachineCommand = "DISK"
	MachinePxeCmd            MachineCommand = "PXE"
	MachineReinstallCmd      MachineCommand = "REINSTALL"
	ChassisIdentifyLEDOnCmd  MachineCommand = "LED-ON"
	ChassisIdentifyLEDOffCmd MachineCommand = "LED-OFF"
	UpdateFirmwareCmd        MachineCommand = "UPDATE-FIRMWARE"
)

type EventType string

// FIXME these constants must move to a single location
const (
	Create  EventType = "create"
	Update  EventType = "update"
	Delete  EventType = "delete"
	Command EventType = "command"
)

func (b *BMCService) outBand(ipmi *IPMI) (hal.OutBand, error) {
	host, portString, found := strings.Cut(ipmi.Address, ":")
	if !found {
		portString = "623"

	}
	port, err := strconv.Atoi(portString)
	if err != nil {
		return nil, fmt.Errorf("unable to convert port to an int %w", err)
	}
	outBand, err := connect.OutBand(host, port, ipmi.User, ipmi.Password, halzap.New(b.log))
	if err != nil {
		return nil, err
	}
	return outBand, nil
}

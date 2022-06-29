package domain

import (
	"net/url"
	"time"
)

type Config struct {
	// Valid log levels are: DEBUG, INFO, WARN, ERROR, FATAL and PANIC
	LogLevel        string        `required:"false" default:"debug" desc:"set log level" split_words:"true"`
	PartitionID     string        `required:"true" desc:"set the partition ID" envconfig:"partition_id"`
	LeaseFile       string        `required:"false" default:"/var/lib/dhcp/dhcpd.leases" desc:"the dhcp lease file to read" split_words:"true"`
	ReportInterval  time.Duration `required:"false" default:"5m" desc:"the interval for periodical reports" split_words:"true"`
	MetalAPIURL     *url.URL      `required:"true" desc:"endpoint for the metal-api" envconfig:"metal_api_url"`
	MetalAPIHMACKey string        `required:"true" desc:"the preshared key for the hmac calculation" envconfig:"metal_api_hmac_key"`
	IpmiPort        int           `required:"false" default:"623" desc:"the ipmi port" split_words:"true"`
	IpmiUser        string        `required:"false" default:"ADMIN" desc:"the ipmi user" split_words:"true"`
	IpmiPassword    string        `required:"false" default:"ADMIN" desc:"the ipmi password" split_words:"true"`
	IgnoreMacs      []string      `required:"false" desc:"mac addresses to ignore" split_words:"true"`

	MQAddress        string        `required:"false" default:"localhost:4161" desc:"set the MQ server address" envconfig:"mq_address"`
	MQCACertFile     string        `required:"false" default:"" desc:"the CA certificate file for verifying MQ certificate" envconfig:"mq_ca_cert_file"`
	MQClientCertFile string        `required:"false" default:"" desc:"the client certificate file for accessing MQ" envconfig:"mq_client_cert_file"`
	MQLogLevel       string        `required:"false" default:"warn" desc:"sets the MQ loglevel (debug, info, warn, error)" envconfig:"mq_loglevel"`
	MachineTopic     string        `required:"false" default:"machine" desc:"set the machine topic name" split_words:"true"`
	MachineTopicTTL  time.Duration `required:"false" default:"30s" desc:"sets the TTL for MachineTopic" envconfig:"machine_topic_ttl"`

	ConsolePort int `required:"false" default:"4444" desc:"defines the port where to listen for incoming console connections from metal-console" envconfig:"console_port"`
}

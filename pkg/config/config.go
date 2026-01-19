package config

import (
	"net/netip"
	"net/url"
	"time"
)

type Config struct {
	// Valid log levels are: DEBUG, INFO, WARN, ERROR, FATAL
	LogLevel    string `required:"false" default:"debug" desc:"set log level" split_words:"true"`
	PartitionID string `required:"true" desc:"set the partition ID" envconfig:"partition_id"`

	// bmc details reporting parameters
	LeaseFile           string        `required:"false" default:"/var/lib/dhcp/dhcpd.leases" desc:"the dhcp lease file to read" split_words:"true"`
	ReportInterval      time.Duration `required:"false" default:"5m" desc:"the interval for periodical reports" split_words:"true"`
	MetalAPIServerURL   *url.URL      `required:"true" desc:"endpoint for the metal-api" envconfig:"metal_apiserver_url"`
	MetalAPIServerToken string        `required:"true" desc:"the preshared key for the hmac calculation" envconfig:"metal_apiserver_token"`
	BMCPort             int           `required:"false" default:"623" desc:"the bmc port" split_words:"true"`
	BMCUser             string        `required:"false" default:"ADMIN" desc:"the bmc user" split_words:"true"`
	BMCPassword         string        `required:"false" default:"ADMIN" desc:"the bmc password" split_words:"true"`
	IgnoreMacs          []string      `required:"false" desc:"mac addresses to ignore" split_words:"true"`
	AllowedCidrs        []string      `required:"false" default:"0.0.0.0/0" desc:"filters dhcp leases" split_words:"true"`

	// Console Proxy parameters
	ConsoleDisabled   bool   `required:"false" desc:"should console be disabled" envconfig:"console_disabled"`
	ConsolePort       int    `required:"false" default:"3333" desc:"defines the port where to listen for incoming console connections from metal-console" envconfig:"console_port"`
	ConsoleCACertFile string `required:"false" default:"ca.pem" desc:"ca cert file" envconfig:"console_ca_cert_file"`
	ConsoleCertFile   string `required:"false" default:"cert.pem" desc:"cert file" envconfig:"console_cert_file"`
	ConsoleKeyFile    string `required:"false" default:"key.pem" desc:"key file" envconfig:"console_key_file"`
}

func (c *Config) Validate() error {
	for _, cidr := range c.AllowedCidrs {
		_, err := netip.ParsePrefix(cidr)
		if err != nil {
			return err
		}
	}
	return nil
}

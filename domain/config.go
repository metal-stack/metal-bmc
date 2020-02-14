package domain

import (
	"fmt"
	"net/url"
	"time"
)

type Config struct {
	// Valid log levels are: DEBUG, INFO, WARN, ERROR, FATAL and PANIC
	LogLevel         string        `required:"false" default:"debug" desc:"set log level" split_words:"true"`
	PartitionID      string        `required:"true" desc:"set the partition ID" envconfig:"partition_id"`
	LeaseFile        string        `required:"false" default:"/var/lib/dhcp/dhcpd.leases" desc:"the dhcp lease file to read" split_words:"true"`
	DebounceInterval time.Duration `required:"false" default:"20s" desc:"the duration to debounce dhcp events" split_words:"true"`
	ReportInterval   time.Duration `required:"false" default:"5m" desc:"the interval for periodical reports" split_words:"true"`
	MetalAPIURL      *url.URL      `required:"true" desc:"endpoint for the metal-api" envconfig:"metal_api_url"`
	MetalAPIHMACKey  string        `required:"true" desc:"the preshared key for the hmac calculation" envconfig:"metal_api_hmac_key"`
	IpmiUser         string        `required:"false" default:"ADMIN" desc:"the ipmi user" split_words:"true"`
	IpmiPassword     string        `required:"false" default:"ADMIN" desc:"the ipmi password" split_words:"true"`
	SumBin           string        `required:"false" default:"/sum" desc:"the sum binary" split_words:"true"`
	IgnoreMacs       []string      `required:"false" desc:"mac addresses to ignore" split_words:"true"`
}

func (c Config) String() string {
	return fmt.Sprintf("loglevel:%s partition:%s leasefile:%s debounce interval:%s report interval:%s metal-api url:%s ipmiuser:%s sum:%s",
		c.LogLevel, c.PartitionID, c.LeaseFile, c.DebounceInterval, c.ReportInterval, c.MetalAPIURL, c.IpmiUser, c.SumBin)
}

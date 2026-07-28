package config

import (
	"testing"

	"github.com/kelseyhightower/envconfig"
	"github.com/metal-stack/metal-bmc/internal/leases"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLeaseFormatFromEnvironment(t *testing.T) {
	tests := []struct {
		name        string
		leaseFormat string
		want        leases.Format
		wantFile    string
		wantErr     string
	}{
		{
			name:     "defaults to the isc-dhcp-server",
			want:     leases.FormatIsc,
			wantFile: "/var/lib/dhcp/dhcpd.leases",
		},
		{
			name:        "kea",
			leaseFormat: "kea",
			want:        leases.FormatKea,
			wantFile:    "/var/lib/dhcp/dhcpd.leases",
		},
		{
			name:        "unsupported format",
			leaseFormat: "dnsmasq",
			wantErr:     `unsupported lease file format "dnsmasq", supported formats are: isc, kea`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("METAL_BMC_PARTITION_ID", "partition")
			t.Setenv("METAL_BMC_METAL_API_URL", "http://localhost:8080")
			t.Setenv("METAL_BMC_METAL_API_HMAC_KEY", "test")
			if tt.leaseFormat != "" {
				t.Setenv("METAL_BMC_LEASE_FORMAT", tt.leaseFormat)
			}

			var cfg Config
			require.NoError(t, envconfig.Process("METAL_BMC", &cfg))

			err := cfg.Validate()
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)

			format, err := cfg.GetLeaseFormat()
			require.NoError(t, err)
			assert.Equal(t, tt.want, format)
			assert.Equal(t, tt.wantFile, cfg.LeaseFile)
		})
	}
}

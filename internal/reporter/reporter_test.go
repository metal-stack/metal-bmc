package reporter

import (
	"log/slog"
	"os"
	"testing"
	"time"

	_ "embed"

	"github.com/google/go-cmp/cmp"
	"github.com/metal-stack/metal-bmc/internal/leases"
	"github.com/metal-stack/metal-bmc/pkg/config"
	"github.com/stretchr/testify/require"
)

//go:embed dhcpd.test.leases
var iscLeaseFile string

// keaLeaseFile contains the same reportable lease as iscLeaseFile
//
//go:embed kea-leases4.test.csv
var keaLeaseFile string

func Test_reporter_getReportItems(t *testing.T) {
	tests := []struct {
		name        string
		leaseFile   string
		leaseFormat leases.Format
		want        []*leases.ReportItem
		err         error
	}{
		{
			name:        "parse isc leases file",
			leaseFile:   iscLeaseFile,
			leaseFormat: leases.FormatIsc,
			want: []*leases.ReportItem{
				{
					Lease: leases.Lease{
						Mac:   "00:00:00:00:00:01",
						Ip:    "10.0.0.1",
						Begin: time.Date(2080, 01, 8, 14, 44, 2, 0, time.UTC),
						End:   time.Date(2080, 01, 10, 14, 44, 2, 0, time.UTC),
					},
				},
			},
		},
		{
			name:        "parse kea leases file",
			leaseFile:   keaLeaseFile,
			leaseFormat: leases.FormatKea,
			want: []*leases.ReportItem{
				{
					Lease: leases.Lease{
						Mac:   "00:00:00:00:00:01",
						Ip:    "10.0.0.1",
						Begin: time.Date(2080, 01, 8, 14, 44, 2, 0, time.UTC),
						End:   time.Date(2080, 01, 10, 14, 44, 2, 0, time.UTC),
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := os.CreateTemp("", "test")
			require.NoError(t, err)
			defer func() {
				err := f.Close()
				require.NoError(t, err)
			}()

			err = os.WriteFile(f.Name(), []byte(tt.leaseFile), 0600)
			require.NoError(t, err)

			r := &reporter{
				cfg: &config.Config{
					LeaseFile:    f.Name(),
					AllowedCidrs: []string{"10.0.0.1/24"},
				},
				log:         slog.Default(),
				leaseFormat: tt.leaseFormat,
			}

			got, err := r.getReportItems()
			if diff := cmp.Diff(tt.err, err); diff != "" {
				t.Errorf("error diff = %s", diff)
				return
			}

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("diff = %s", diff)
			}
		})
	}
}

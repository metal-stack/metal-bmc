package leases

import (
	"log/slog"
	"os"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilterActive(t *testing.T) {
	l, err := parseLeasesFile(slog.Default(), sampleLeaseContent, FormatIsc)
	require.NoError(t, err)
	assert.Equal(t, Leases{}, l.FilterActive())
}

func TestLatestByMac(t *testing.T) {
	l1 := Lease{
		Mac: "aa:aa",
		End: time.Now(),
	}
	l2 := Lease{
		Mac: "bb:bb",
		End: time.Now(),
	}
	l3 := Lease{
		Mac: "aa:aa",
		End: time.Now().AddDate(0, 0, -1),
	}
	leases := Leases{l1, l2, l3}
	byMac := leases.LatestByMac()
	expected := map[string]Lease{"aa:aa": l1, "bb:bb": l2}
	assert.Equal(t, expected, byMac)
}

func TestReadLeases(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		format  Format
		wantErr string
	}{
		{
			name:   "isc",
			data:   sampleLeaseContent,
			format: FormatIsc,
		},
		{
			name:   "kea",
			data:   sampleKeaLeaseContent,
			format: FormatKea,
		},
		{
			name:    "unsupported format",
			data:    sampleLeaseContent,
			format:  Format("dnsmasq"),
			wantErr: `unable to parse lease file: unsupported lease file format: "dnsmasq"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			leaseFile := path.Join(t.TempDir(), "leases")
			require.NoError(t, os.WriteFile(leaseFile, []byte(tt.data), 0600))

			l, err := ReadLeases(slog.Default(), leaseFile, tt.format)
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)

			expected := Leases{
				{
					Mac:   "ac:1f:6b:35:ac:62",
					Ip:    "192.168.2.27",
					Begin: time.Date(2019, 06, 27, 13, 30, 21, 0, time.UTC),
					End:   time.Date(2019, 06, 27, 13, 40, 21, 0, time.UTC),
				},
				{
					Mac:   "ac:1f:6b:35:ab:2d",
					Ip:    "192.168.2.30",
					Begin: time.Date(2019, 06, 27, 6, 40, 06, 0, time.UTC),
					End:   time.Date(2019, 06, 27, 6, 50, 06, 0, time.UTC),
				},
			}
			assert.Equal(t, expected, l)
		})
	}
}

func TestReadLeasesOfNonExistingFile(t *testing.T) {
	_, err := ReadLeases(slog.Default(), path.Join(t.TempDir(), "does-not-exist"), FormatIsc)
	require.ErrorIs(t, err, os.ErrNotExist)
}

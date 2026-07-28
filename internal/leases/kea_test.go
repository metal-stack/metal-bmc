package leases

import (
	"fmt"
	"log/slog"
	"strconv"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/metal-stack/metal-lib/pkg/testcommon"
)

// sampleKeaLeaseContent describes the same leases as sampleLeaseContent,
// 1561642821 is 2019/06/27 13:40:21 UTC and 1561618206 is 2019/06/27 06:50:06 UTC.
var sampleKeaLeaseContent = `address,hwaddr,client_id,valid_lifetime,expire,subnet_id,fqdn_fwd,fqdn_rev,hostname,state,user_context,pool_id
192.168.2.27,ac:1f:6b:35:ac:62,,600,1561642821,1,0,0,,0,,0
192.168.2.30,ac:1f:6b:35:ab:2d,,600,1561618206,1,0,0,,0,,0
`

func Test_parseKeaLeasesFile(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		want    Leases
		wantErr error
	}{
		{
			name: "empty file",
		},
		{
			name: "header line only",
			data: "address,hwaddr,client_id,valid_lifetime,expire,subnet_id,fqdn_fwd,fqdn_rev,hostname,state,user_context,pool_id\n",
		},
		{
			name:    "missing mandatory column",
			data:    "address,client_id,valid_lifetime,expire\n192.168.2.27,,600,1561642821\n",
			wantErr: fmt.Errorf(`missing "hwaddr" column in header line: address,client_id,valid_lifetime,expire`),
		},
		{
			name: "invalid valid lifetime",
			data: `address,hwaddr,client_id,valid_lifetime,expire
192.168.2.27,ac:1f:6b:35:ac:62,,forever,1561642821
`,
			wantErr: fmt.Errorf("invalid valid_lifetime on line 2: %w", &strconv.NumError{Func: "ParseInt", Num: "forever", Err: strconv.ErrSyntax}),
		},
		{
			name: "invalid expire",
			data: `address,hwaddr,client_id,valid_lifetime,expire
192.168.2.27,ac:1f:6b:35:ac:62,,600,1561642821
192.168.2.30,ac:1f:6b:35:ab:2d,,600,tomorrow
`,
			wantErr: fmt.Errorf("invalid expire on line 3: %w", &strconv.NumError{Func: "ParseInt", Num: "tomorrow", Err: strconv.ErrSyntax}),
		},
		{
			name: "the last entry of an address wins because the memfile is written append-only",
			data: `address,hwaddr,client_id,valid_lifetime,expire,subnet_id,fqdn_fwd,fqdn_rev,hostname,state,user_context,pool_id
192.168.2.27,ac:1f:6b:35:ac:62,,600,1561642821,1,0,0,,0,,0
192.168.2.27,ac:1f:6b:35:ac:62,,600,1561643421,1,0,0,,0,,0
`,
			want: Leases{
				{
					Mac:   "ac:1f:6b:35:ac:62",
					Ip:    "192.168.2.27",
					Begin: time.Date(2019, 06, 27, 13, 40, 21, 0, time.UTC),
					End:   time.Date(2019, 06, 27, 13, 50, 21, 0, time.UTC),
				},
			},
		},
		{
			name: "skip when the lease was removed with a valid lifetime of zero",
			data: `address,hwaddr,client_id,valid_lifetime,expire,subnet_id,fqdn_fwd,fqdn_rev,hostname,state,user_context,pool_id
192.168.2.27,ac:1f:6b:35:ac:62,,600,1561642821,1,0,0,,0,,0
192.168.2.30,ac:1f:6b:35:ab:2d,,600,1561618206,1,0,0,,0,,0
192.168.2.27,ac:1f:6b:35:ac:62,,0,1561642821,1,0,0,,0,,0
`,
			want: Leases{
				{
					Mac:   "ac:1f:6b:35:ab:2d",
					Ip:    "192.168.2.30",
					Begin: time.Date(2019, 06, 27, 6, 40, 06, 0, time.UTC),
					End:   time.Date(2019, 06, 27, 6, 50, 06, 0, time.UTC),
				},
			},
		},
		{
			name: "skip when the lease is declined, expired-reclaimed or released",
			data: `address,hwaddr,client_id,valid_lifetime,expire,subnet_id,fqdn_fwd,fqdn_rev,hostname,state,user_context,pool_id
192.168.2.27,ac:1f:6b:35:ac:62,,600,1561642821,1,0,0,,1,,0
192.168.2.28,ac:1f:6b:35:ac:63,,600,1561642821,1,0,0,,2,,0
192.168.2.29,ac:1f:6b:35:ac:64,,600,1561642821,1,0,0,,4,,0
192.168.2.30,ac:1f:6b:35:ab:2d,,600,1561618206,1,0,0,,0,,0
`,
			want: Leases{
				{
					Mac:   "ac:1f:6b:35:ab:2d",
					Ip:    "192.168.2.30",
					Begin: time.Date(2019, 06, 27, 6, 40, 06, 0, time.UTC),
					End:   time.Date(2019, 06, 27, 6, 50, 06, 0, time.UTC),
				},
			},
		},
		{
			name: "skip when mac address is missing",
			data: `address,hwaddr,client_id,valid_lifetime,expire,subnet_id,fqdn_fwd,fqdn_rev,hostname,state,user_context,pool_id
192.168.2.27,,,600,1561642821,1,0,0,,0,,0
192.168.2.30,ac:1f:6b:35:ab:2d,,600,1561618206,1,0,0,,0,,0
`,
			want: Leases{
				{
					Mac:   "ac:1f:6b:35:ab:2d",
					Ip:    "192.168.2.30",
					Begin: time.Date(2019, 06, 27, 6, 40, 06, 0, time.UTC),
					End:   time.Date(2019, 06, 27, 6, 50, 06, 0, time.UTC),
				},
			},
		},
		{
			name: "columns are looked up by name, so older kea versions and a different order work as well",
			data: `expire,valid_lifetime,hwaddr,address,client_id,subnet_id,fqdn_fwd,fqdn_rev,hostname
1561618206,600,ac:1f:6b:35:ab:2d,192.168.2.30,,1,0,0,
`,
			want: Leases{
				{
					Mac:   "ac:1f:6b:35:ab:2d",
					Ip:    "192.168.2.30",
					Begin: time.Date(2019, 06, 27, 6, 40, 06, 0, time.UTC),
					End:   time.Date(2019, 06, 27, 6, 50, 06, 0, time.UTC),
				},
			},
		},
		{
			name: "a hostname containing a comma is quoted",
			data: `address,hwaddr,client_id,valid_lifetime,expire,subnet_id,fqdn_fwd,fqdn_rev,hostname,state,user_context,pool_id
192.168.2.30,ac:1f:6b:35:ab:2d,,600,1561618206,1,0,0,"host,with,comma",0,,0
`,
			want: Leases{
				{
					Mac:   "ac:1f:6b:35:ab:2d",
					Ip:    "192.168.2.30",
					Begin: time.Date(2019, 06, 27, 6, 40, 06, 0, time.UTC),
					End:   time.Date(2019, 06, 27, 6, 50, 06, 0, time.UTC),
				},
			},
		},
		{
			name: "real example",
			data: sampleKeaLeaseContent,
			want: Leases{
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
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotErr := parseLeasesFile(slog.Default(), tt.data, FormatKea)
			if diff := cmp.Diff(tt.wantErr, gotErr, testcommon.ErrorStringComparer()); diff != "" {
				t.Errorf("error diff = %s", diff)
				return
			}

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("diff = %s", diff)
			}
		})
	}
}

// Test_parseLeasesFile_bothFormatsYieldTheSameLeases makes sure that it does not matter
// for the rest of the application which dhcp server the leases were read from.
func Test_parseLeasesFile_bothFormatsYieldTheSameLeases(t *testing.T) {
	isc, err := parseLeasesFile(slog.Default(), sampleLeaseContent, FormatIsc)
	if err != nil {
		t.Fatalf("unable to parse isc leases: %s", err)
	}

	kea, err := parseLeasesFile(slog.Default(), sampleKeaLeaseContent, FormatKea)
	if err != nil {
		t.Fatalf("unable to parse kea leases: %s", err)
	}

	if diff := cmp.Diff(isc, kea); diff != "" {
		t.Errorf("diff = %s", diff)
	}
}

func Test_parseLeasesFile_unsupportedFormat(t *testing.T) {
	_, gotErr := parseLeasesFile(slog.Default(), sampleLeaseContent, Format("dnsmasq"))

	wantErr := fmt.Errorf(`unsupported lease file format: "dnsmasq"`)
	if diff := cmp.Diff(wantErr, gotErr, testcommon.ErrorStringComparer()); diff != "" {
		t.Errorf("error diff = %s", diff)
	}
}

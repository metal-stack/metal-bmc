package leases

import (
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/metal-stack/metal-lib/pkg/testcommon"
)

var sampleLeaseContent = `
lease 192.168.2.27 {
	starts 4 2019/06/27 13:30:21;
	ends 4 2019/06/27 13:40:21;
	cltt 4 2019/06/27 13:30:21;
	binding state active;
	next binding state free;
	rewind binding state free;
	hardware ethernet ac:1f:6b:35:ac:62;
	uid "\001\254\037k5\254b";
	set vendor-class-identifier = "udhcp 1.23.1";
	option agent.circuit-id "eqx-mu4-r02mgmtleaf:Eth6(Port6)";
	option agent.remote-id "e0:01:a6:db:04:3f";
}
lease 192.168.2.30 {
	starts 4 2019/06/27 06:40:06;
	ends 4 2019/06/27 06:50:06;
	cltt 4 2019/06/27 06:40:06;
	binding state active;
	next binding state free;
	rewind binding state free;
	hardware ethernet ac:1f:6b:35:ab:2d;
	uid "\001\254\037k5\253-";
	set vendor-class-identifier = "udhcp 1.23.1";
	option agent.circuit-id "eqx-mu4-r02mgmtleaf:Eth6(Port6)";
	option agent.remote-id "e0:01:a6:db:04:3f";
}
`

func Test_parseLeasesFile(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		want    Leases
		wantErr error
	}{
		{
			name: "not closed entry",
			data: `lease 1.2.3.4 {
				starts 4 2019/06/27 06:40:06;
				ends 4 2019/06/27 06:50:06;
				hardware ethernet ac:1f:6b:35:ab:2d;
			`,
			wantErr: fmt.Errorf("lease entry was not closed"),
		},
		{
			name: "invalid start date",
			data: `lease 1.2.3.4 {
				starts 2019/06/27 06:40:06;
				ends 4 2019/06/27 06:50:06;
				hardware ethernet ac:1f:6b:35:ab:2d;
			}`,
			wantErr: fmt.Errorf(`expecting "starts <whatever-number> <date> <time>;" on line 2, got: starts 2019/06/27 06:40:06;`),
		},
		{
			name:    "invalid opening line",
			data:    `lease 1.2.3.4 { starts 2019/06/27 06:40:06; }`,
			wantErr: fmt.Errorf(`expecting "lease <ip> {" on line 1, got: lease 1.2.3.4 { starts 2019/06/27 06:40:06; }`),
		},
		{
			name: "invalid hardware format",
			data: `lease 1.2.3.4 {
				starts 4 2019/06/27 06:40:06;
				ends 4 2019/06/27 06:50:06;
				hardware foo bar ac:1f:6b:35:ab:2d;
			}`,
			wantErr: fmt.Errorf(`expecting "hardware ethernet <mac>;" on line 4, got: hardware foo bar ac:1f:6b:35:ab:2d;`),
		},
		{
			name: "skip when mac address is missing",
			data: `lease 1.2.3.4 {
				starts 4 2019/06/27 06:40:06;
				ends 4 2019/06/27 06:50:06;
			}
			lease 1.2.3.5 {
				starts 4 2019/06/27 06:40:06;
				ends 4 2019/06/27 06:50:06;
				hardware ethernet ac:1f:6b:35:ab:2d;
			}`,
			want: Leases{
				{
					Mac:   "ac:1f:6b:35:ab:2d",
					Ip:    "1.2.3.5",
					Begin: time.Date(2019, 06, 27, 6, 40, 06, 0, time.UTC),
					End:   time.Date(2019, 06, 27, 6, 50, 06, 0, time.UTC),
				},
			},
			wantErr: nil,
		},
		{
			name: "real example",
			data: sampleLeaseContent,
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
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotErr := parseLeasesFile(slog.Default(), tt.data)
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

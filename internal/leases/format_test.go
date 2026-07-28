package leases

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFormat(t *testing.T) {
	tests := []struct {
		format  string
		want    Format
		wantErr string
	}{
		{format: "isc", want: FormatIsc},
		{format: "kea", want: FormatKea},
		{format: " KEA ", want: FormatKea},
		{format: "dnsmasq", wantErr: `unsupported lease file format "dnsmasq", supported formats are: isc, kea`},
		{format: "", wantErr: `unsupported lease file format "", supported formats are: isc, kea`},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			got, err := ParseFormat(tt.format)
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

package leases

import (
	"fmt"
	"slices"
	"strings"
)

// Format describes the lease file format of the dhcp server in use.
type Format string

const (
	// FormatIsc is the lease file format written by the isc-dhcp-server, usually /var/lib/dhcp/dhcpd.leases.
	FormatIsc Format = "isc"
	// FormatKea is the memfile lease file format written by the kea-dhcp-server, usually /var/lib/kea/kea-leases4.csv.
	FormatKea Format = "kea"
)

// Formats contains all supported lease file formats.
var Formats = []Format{FormatIsc, FormatKea}

// ParseFormat converts the given string into a supported lease file Format.
func ParseFormat(format string) (Format, error) {
	f := Format(strings.ToLower(strings.TrimSpace(format)))
	if !slices.Contains(Formats, f) {
		var supported []string
		for _, s := range Formats {
			supported = append(supported, string(s))
		}
		return "", fmt.Errorf("unsupported lease file format %q, supported formats are: %s", format, strings.Join(supported, ", "))
	}
	return f, nil
}

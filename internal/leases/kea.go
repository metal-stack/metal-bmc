package leases

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"time"
)

const (
	keaColumnAddress       = "address"
	keaColumnHwAddr        = "hwaddr"
	keaColumnValidLifetime = "valid_lifetime"
	keaColumnExpire        = "expire"
	keaColumnState         = "state"

	// keaStateDefault is the state of a lease that is actually assigned to a client,
	// every other state (declined, expired-reclaimed, released) describes a lease
	// that is not in use.
	keaStateDefault = "0"
)

// parseKeaLeasesFile parses the memfile lease file format of the kea-dhcp-server.
//
// The memfile is a csv file whose first line contains the column names. It is written
// append-only, so the same address can occur several times, whereby the last entry
// wins. A lease is removed by appending it with a valid lifetime of zero.
func parseKeaLeasesFile(log *slog.Logger, data string) (Leases, error) {
	reader := csv.NewReader(strings.NewReader(data))
	// kea appended columns over the versions and does not rewrite the memfile on an
	// upgrade, so the number of fields can vary from entry to entry
	reader.FieldsPerRecord = -1

	header, err := reader.Read()
	if errors.Is(err, io.EOF) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("unable to read header line: %w", err)
	}

	columns := map[string]int{}
	for i, name := range header {
		columns[strings.TrimSpace(name)] = i
	}
	for _, required := range []string{keaColumnAddress, keaColumnHwAddr, keaColumnValidLifetime, keaColumnExpire} {
		if _, ok := columns[required]; !ok {
			return nil, fmt.Errorf("missing %q column in header line: %s", required, strings.Join(header, ","))
		}
	}

	var (
		// preserves the order in which the addresses appear in the lease file
		addresses []string
		seen      = map[string]bool{}
		byAddress = map[string]Lease{}
	)

	for {
		entry, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("unable to read lease entry: %w", err)
		}

		// the line of the first field, which is also correct for quoted fields spanning multiple lines
		line, _ := reader.FieldPos(0)

		field := func(name string) string {
			i, ok := columns[name]
			if !ok || i >= len(entry) {
				return ""
			}
			return strings.TrimSpace(entry[i])
		}

		address := field(keaColumnAddress)
		if address == "" {
			log.Warn("incomplete lease entry (missing ip address), skipping entry", "line", line)
			continue
		}
		if !seen[address] {
			seen[address] = true
			addresses = append(addresses, address)
		}

		validLifetime, err := strconv.ParseInt(field(keaColumnValidLifetime), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid %s on line %d: %w", keaColumnValidLifetime, line, err)
		}

		expire, err := strconv.ParseInt(field(keaColumnExpire), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid %s on line %d: %w", keaColumnExpire, line, err)
		}

		// kea removes a lease by appending it with a valid lifetime of zero, the state
		// column is not present in very old kea versions and defaults to the assigned state then
		state := field(keaColumnState)
		if validLifetime == 0 || (state != "" && state != keaStateDefault) {
			log.Debug("lease entry is not assigned to a client, skipping entry", "line", line, "ip", address, "state", state)
			delete(byAddress, address)
			continue
		}

		mac := field(keaColumnHwAddr)
		if mac == "" {
			log.Warn("incomplete lease entry (missing mac address), skipping entry", "line", line)
			delete(byAddress, address)
			continue
		}

		end := time.Unix(expire, 0).UTC()
		byAddress[address] = Lease{
			Mac:   mac,
			Ip:    address,
			Begin: end.Add(-time.Duration(validLifetime) * time.Second),
			End:   end,
		}
	}

	var leases Leases
	for _, address := range addresses {
		lease, ok := byAddress[address]
		if !ok {
			continue
		}
		leases = append(leases, lease)
	}

	return leases, nil
}

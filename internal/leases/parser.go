package leases

import (
	"fmt"
	"log/slog"
	"net/netip"
	"strings"
	"time"
)

const (
	leaseDateFormat = "2006/01/02 15:04:05"
)

func parseLeasesFile(log *slog.Logger, data string) (Leases, error) {
	var (
		leases  Leases
		current *Lease
	)

	for i, line := range strings.Split(data, "\n") {
		tokens := strings.Fields(line)
		if len(tokens) == 0 {
			continue
		}

		switch tokens[0] {
		case "lease":
			// lease 1.2.3.4 {
			if len(tokens) != 3 {
				return nil, fmt.Errorf(`expecting "lease <ip> {" on line %d, got: %s`, i+1, line)
			}

			if tokens[2] != "{" {
				return nil, fmt.Errorf("missing opening brace on line %d: %s", i+1, line)
			}

			if _, err := netip.ParseAddr(tokens[1]); err != nil {
				return nil, fmt.Errorf("invalid ip address on line %d: %w", i+1, err)
			}

			current = &Lease{
				Ip: tokens[1],
			}

		case "}":
			if current == nil {
				return nil, fmt.Errorf("unexpected closing brace on line %d: %s", i+1, line)
			}

			switch {
			case current.Begin.IsZero(), current.End.IsZero():
				log.Warn("incomplete lease entry (missing begin and end time), skipping entry", "line", i+1)
				continue
			case current.Ip == "":
				log.Warn("incomplete lease entry (missing ip address), skipping entry", "line", i+1)
				continue
			case current.Mac == "":
				log.Warn("incomplete lease entry (missing mac address), skipping entry", "line", i+1)
				continue
			default:
				leases = append(leases, *current)
				current = nil
			}

		case "starts", "ends":
			// starts 5 2026/01/09 12:35:39;
			if current == nil {
				return nil, fmt.Errorf("unexpected date field on line %d: %s", i+1, line)
			}

			if len(tokens) != 4 {
				return nil, fmt.Errorf(`expecting "%s <whatever-number> <date> <time>;" on line %d, got: %s`, tokens[0], i+1, line)
			}

			if !strings.HasSuffix(tokens[3], ";") {
				return nil, fmt.Errorf("missing semicolon on line %d: %s", i+1, line)
			}

			tokens[3] = strings.TrimRight(tokens[3], ";")

			t, err := time.Parse(leaseDateFormat, tokens[2]+" "+tokens[3])
			if err != nil {
				return nil, fmt.Errorf("invalid time format on line %d: %w", i+1, err)
			}

			if tokens[0] == "starts" {
				current.Begin = t
			} else {
				current.End = t
			}

		case "hardware":
			//  hardware ethernet 50:7c:6f:3e:8d:59;
			if current == nil {
				return nil, fmt.Errorf("unexpected hardware field on line %d: %s", i+1, line)
			}

			if len(tokens) != 3 {
				return nil, fmt.Errorf(`expecting "hardware ethernet <mac>;" on line %d, got: %s`, i+1, line)
			}

			if tokens[1] != "ethernet" {
				continue
			}

			if !strings.HasSuffix(tokens[2], ";") {
				return nil, fmt.Errorf("missing semicolon on line %d: %s", i+1, line)
			}

			current.Mac = strings.TrimRight(tokens[2], ";")
		}
	}

	return leases, nil
}

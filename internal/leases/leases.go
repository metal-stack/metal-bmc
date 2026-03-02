package leases

import (
	"fmt"
	"log/slog"
	"os"
	"time"
)

func (l Leases) FilterActive() Leases {
	active := Leases{}
	now := time.Now()
	for _, lease := range l {
		if lease.End.Before(now) {
			continue
		}
		active = append(active, lease)
	}
	return active
}

func (l Leases) LatestByMac() map[string]Lease {
	byMac := map[string]Lease{}
	for _, lease := range l {
		if e, ok := byMac[lease.Mac]; !ok {
			byMac[lease.Mac] = lease
		} else if lease.End.After(e.End) {
			byMac[lease.Mac] = lease
		}
	}
	return byMac
}

func ReadLeases(log *slog.Logger, leaseFilePath string) (Leases, error) {
	data, err := os.ReadFile(leaseFilePath)
	if err != nil {
		return nil, err
	}

	leases, err := parseLeasesFile(log, string(data))
	if err != nil {
		return nil, fmt.Errorf("unable to parse lease file: %w", err)
	}

	return leases, nil
}

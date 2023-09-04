package leases

import (
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

func ReadLeases(leaseFile string) (Leases, error) {
	leasesContent := mustRead(leaseFile)
	return parse(leasesContent)
}

func mustRead(name string) string {
	c, err := os.ReadFile(name)
	if err != nil {
		panic(err)
	}
	return string(c)
}

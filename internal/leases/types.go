package leases

import "time"

type Lease struct {
	Mac   string
	Ip    string
	Begin time.Time
	End   time.Time
}

type Leases []Lease

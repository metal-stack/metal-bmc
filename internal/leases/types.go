package leases

import (
	"time"

	apiv2 "github.com/metal-stack/api/go/metalstack/api/v2"
)

type Lease struct {
	Mac   string
	Ip    string
	Begin time.Time
	End   time.Time
}

type Leases []Lease

type ReportItem struct {
	Lease Lease
	apiv2.MachineBMCReport
}

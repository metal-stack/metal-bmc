package leases

import (
	"github.com/metal-stack/metal-go/api/models"
	"time"
)

type Lease struct {
	Mac   string
	Ip    string
	Begin time.Time
	End   time.Time
}

type Leases []Lease

type ReportItem struct {
	Lease
	FRU         *models.V1MachineFru
	BmcVersion  *string
	BiosVersion *string
}

package leases

import (
	"time"

	"github.com/metal-stack/metal-go/api/models"
)

type Lease struct {
	Mac   string
	Ip    string
	Begin time.Time
	End   time.Time
}

type Leases []Lease

type ReportItem struct {
	Lease         Lease
	UUID          *string
	BmcVersion    *string
	BiosVersion   *string
	FRU           *models.V1MachineFru
	Powerstate    *string
	IndicatorLED  *string
	PowerMetric   *models.V1PowerMetric
	PowerSupplies []*models.V1PowerSupply
}

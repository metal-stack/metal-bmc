package leases

import (
	"log/slog"
	"time"

	"github.com/metal-stack/metal-go/api/models"
)

type Lease struct {
	Mac string
	Ip  string
	End time.Time
}

type Leases []Lease

type ReportItem struct {
	Lease
	Log          *slog.Logger
	UUID         *string
	BmcVersion   *string
	BiosVersion  *string
	FRU          *models.V1MachineFru
	Powerstate   *string
	IndicatorLED *string
	PowerMetric  *models.V1PowerMetric
}

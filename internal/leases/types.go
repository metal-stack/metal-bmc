package leases

import (
	"time"

	"github.com/metal-stack/metal-go/api/models"
	"go.uber.org/zap"
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
	Log          *zap.SugaredLogger
	UUID         *string
	BmcVersion   *string
	BiosVersion  *string
	FRU          *models.V1MachineFru
	Powerstate   *string
	IndicatorLED *string
	PowerMetric  *models.V1PowerMetric
}

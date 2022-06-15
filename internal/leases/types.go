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
}

func NewReportItem(l Lease, log *zap.SugaredLogger) *ReportItem {
	return &ReportItem{
		Lease: l,
		Log:   log,
	}
}

func (i *ReportItem) MacContainedIn(macs []string) bool {
	for _, m := range macs {
		if m == i.Mac {
			return true
		}
	}
	return false
}

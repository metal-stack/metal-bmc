package leases

import (
	"github.com/metal-stack/metal-go/api/models"
	"go.uber.org/zap"
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
	Log         *zap.SugaredLogger
	UUID        *string
	BmcVersion  *string
	BiosVersion *string
	FRU         *models.V1MachineFru
}

func NewReportItem(l Lease, log *zap.SugaredLogger) *ReportItem {
	return &ReportItem{
		Lease: l,
		Log:   log,
	}
}

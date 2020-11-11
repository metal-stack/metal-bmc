package leases

import (
	"github.com/metal-stack/bmc-catcher/domain"
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
	Config      domain.Config
	Log         *zap.SugaredLogger
	FRU         *models.V1MachineFru
	BmcVersion  *string
	BiosVersion *string
}

func NewReportItem(l Lease, cfg domain.Config, log *zap.SugaredLogger) *ReportItem {
	return &ReportItem{
		Lease:  l,
		Config: cfg,
		Log:    log,
	}
}

package leases

import (
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilterActive(t *testing.T) {
	l, err := parseLeasesFile(slog.Default(), sampleLeaseContent)
	require.NoError(t, err)
	assert.Equal(t, Leases{}, l.FilterActive())
}

func TestLatestByMac(t *testing.T) {
	l1 := Lease{
		Mac: "aa:aa",
		End: time.Now(),
	}
	l2 := Lease{
		Mac: "bb:bb",
		End: time.Now(),
	}
	l3 := Lease{
		Mac: "aa:aa",
		End: time.Now().AddDate(0, 0, -1),
	}
	leases := Leases{l1, l2, l3}
	byMac := leases.LatestByMac()
	expected := map[string]Lease{"aa:aa": l1, "bb:bb": l2}
	assert.Equal(t, expected, byMac)
}

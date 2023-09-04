package leases

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFilterActive(t *testing.T) {
	assert := assert.New(t)
	l, err := parse(LEASES_CONTENT)
	assert.NoError(err)
	assert.Equal(Leases{}, l.FilterActive())
}

func TestLatestByMac(t *testing.T) {
	assert := assert.New(t)
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
	assert.Equal(expected, byMac)
}

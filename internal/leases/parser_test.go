package leases

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var sampleLeaseContent = `address,hwaddr,client_id,valid_lifetime,expire,subnet_id,fqdn_fwd,fqdn_rev,hostname,state,user_context
192.168.2.27,ac:1f:6b:35:ac:62,01:ac:1f:6b:35:ac:62,3600,1593243021,1,0,0,,1,
192.168.2.30,ac:1f:6b:35:ab:2d,01:ac:1f:6b:35:ab:2d,3600,1593243006,1,0,0,,1,
`

func TestParse(t *testing.T) {
	l, err := parse(strings.NewReader(sampleLeaseContent))
	require.NoError(t, err)

	lease1 := Lease{
		Mac: "ac:1f:6b:35:ac:62",
		Ip:  "192.168.2.27",
		End: time.Unix(1593243021, 0),
	}

	lease2 := Lease{
		Mac: "ac:1f:6b:35:ab:2d",
		Ip:  "192.168.2.30",
		End: time.Unix(1593243006, 0),
	}

	assert.Equal(t, Leases{lease1, lease2}, l)
}

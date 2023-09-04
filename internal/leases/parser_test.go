package leases

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var LEASES_CONTENT = `
lease 192.168.2.27 {
	starts 4 2019/06/27 13:30:21;
	ends 4 2019/06/27 13:40:21;
	cltt 4 2019/06/27 13:30:21;
	binding state active;
	next binding state free;
	rewind binding state free;
	hardware ethernet ac:1f:6b:35:ac:62;
	uid "\001\254\037k5\254b";
	set vendor-class-identifier = "udhcp 1.23.1";
}
lease 192.168.2.30 {
	starts 4 2019/06/27 06:40:06;
	ends 4 2019/06/27 06:50:06;
	cltt 4 2019/06/27 06:40:06;
	binding state active;
	next binding state free;
	rewind binding state free;
	hardware ethernet ac:1f:6b:35:ab:2d;
	uid "\001\254\037k5\253-";
	set vendor-class-identifier = "udhcp 1.23.1";
}
`

func TestParse(t *testing.T) {
	assert := assert.New(t)
	l, err := parse(LEASES_CONTENT)
	assert.NoError(err)

	b, _ := time.Parse(DATE_FORMAT, "2019/06/27 13:30:21")
	e, _ := time.Parse(DATE_FORMAT, "2019/06/27 13:40:21")
	lease1 := Lease{
		Mac:   "ac:1f:6b:35:ac:62",
		Ip:    "192.168.2.27",
		Begin: b,
		End:   e,
	}

	b, _ = time.Parse(DATE_FORMAT, "2019/06/27 06:40:06")
	e, _ = time.Parse(DATE_FORMAT, "2019/06/27 06:50:06")
	lease2 := Lease{
		Mac:   "ac:1f:6b:35:ab:2d",
		Ip:    "192.168.2.30",
		Begin: b,
		End:   e,
	}

	assert.Equal(Leases{lease1, lease2}, l)
}

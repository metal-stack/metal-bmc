package leases

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// this output can be achieved with dhcp-lease-list --parsable
// this is possible if isc-dhcp-server package is installed in the metal-bmc container
const leaseListOutput = `
To get manufacturer names please download http://standards.ieee.org/regauth/oui/oui.txt to /usr/local/etc/oui.txt                                                                                                                                                                
MAC 50:7c:6f:3e:91:11 IP 10.255.6.168 HOSTNAME -NA- BEGIN 2023-08-02 15:03:16 END 2023-08-02 15:05:16 MANUFACTURER -NA-
MAC 50:7c:6f:3e:89:63 IP 10.255.6.134 HOSTNAME -NA- BEGIN 2023-08-02 18:58:10 END 2023-08-02 21:26:16 MANUFACTURER -NA-
MAC 50:7c:6f:3e:89:91 IP 10.255.6.154 HOSTNAME -NA- BEGIN 2023-08-03 16:57:25 END 2023-08-04 08:28:23 MANUFACTURER -NA-
MAC 50:7c:6f:3e:82:37 IP 10.255.6.138 HOSTNAME -NA- BEGIN 2023-08-03 12:05:55 END 2023-08-05 12:05:55 MANUFACTURER -NA-
MAC 50:7c:6f:3e:94:4d IP 10.255.6.169 HOSTNAME -NA- BEGIN 2023-08-31 03:04:41 END 2023-08-31 03:06:41 MANUFACTURER -NA-
MAC 50:7c:6f:3e:90:6b IP 10.255.6.144 HOSTNAME -NA- BEGIN 2023-08-31 07:52:43 END 2023-08-31 07:54:43 MANUFACTURER -NA-
MAC 50:7c:6f:3e:89:a7 IP 10.255.6.153 HOSTNAME -NA- BEGIN 2023-08-31 07:52:44 END 2023-08-31 07:54:44 MANUFACTURER -NA-
MAC 50:7c:6f:3e:80:f7 IP 10.255.6.143 HOSTNAME -NA- BEGIN 2023-08-31 23:34:28 END 2023-08-31 23:36:28 MANUFACTURER -NA-
MAC 50:7c:6f:3e:8f:e5 IP 10.255.6.141 HOSTNAME -NA- BEGIN 2023-08-31 02:04:25 END 2023-09-01 12:08:46 MANUFACTURER -NA-
MAC 50:7c:6f:22:92:80 IP 10.255.6.234 HOSTNAME -NA- BEGIN 2023-09-01 13:01:28 END 2023-09-01 13:03:28 MANUFACTURER -NA-
MAC e8:eb:d3:c1:cc:38 IP 10.255.6.235 HOSTNAME -NA- BEGIN 2023-09-01 13:02:05 END 2023-09-01 13:04:05 MANUFACTURER -NA-
MAC 50:7c:6f:22:93:ba IP 10.255.6.237 HOSTNAME -NA- BEGIN 2023-09-01 14:18:28 END 2023-09-01 14:20:28 MANUFACTURER -NA-
MAC 50:7c:6f:3e:81:05 IP 10.255.6.151 HOSTNAME -NA- BEGIN 2023-08-31 05:06:38 END 2023-09-02 05:06:38 MANUFACTURER -NA-
MAC 50:7c:6f:3e:8e:97 IP 10.255.6.190 HOSTNAME -NA- BEGIN 2023-08-31 06:07:13 END 2023-09-02 06:07:13 MANUFACTURER -NA-
MAC 50:7c:6f:22:90:b2 IP 10.255.6.176 HOSTNAME -NA- BEGIN 2023-08-31 19:53:24 END 2023-09-02 19:53:24 MANUFACTURER -NA-
MAC 50:7c:6f:22:90:b2 IP 10.255.6.155 HOSTNAME -NA- BEGIN 2023-08-31 19:54:42 END 2023-09-02 19:54:42 MANUFACTURER -NA-
MAC 50:7c:6f:22:92:f6 IP 10.255.6.210 HOSTNAME -NA- BEGIN 2023-09-03 01:25:01 END 2023-09-03 01:27:01 MANUFACTURER -NA-
MAC 50:7c:6f:3e:8e:33 IP 10.255.6.137 HOSTNAME -NA- BEGIN 2023-09-03 02:41:51 END 2023-09-03 02:43:51 MANUFACTURER -NA-
MAC 50:7c:6f:3e:8f:c3 IP 10.255.6.152 HOSTNAME -NA- BEGIN 2023-09-03 03:59:35 END 2023-09-03 04:01:35 MANUFACTURER -NA-
MAC 50:7c:6f:3e:8d:91 IP 10.255.6.193 HOSTNAME -NA- BEGIN 2023-09-03 04:01:14 END 2023-09-03 04:03:14 MANUFACTURER -NA-
MAC 50:7c:6f:3e:89:9b IP 10.255.6.161 HOSTNAME -NA- BEGIN 2023-09-02 14:10:40 END 2023-09-03 07:13:41 MANUFACTURER -NA-
MAC 50:7c:6f:3e:88:4d IP 10.255.6.164 HOSTNAME -NA- BEGIN 2023-09-02 19:07:27 END 2023-09-03 09:15:27 MANUFACTURER -NA-
MAC 50:7c:6f:22:91:36 IP 10.255.6.244 HOSTNAME -NA- BEGIN 2023-09-01 13:32:33 END 2023-09-03 13:32:33 MANUFACTURER -NA-
MAC 50:7c:6f:22:91:36 IP 10.255.6.240 HOSTNAME -NA- BEGIN 2023-09-01 13:33:51 END 2023-09-03 13:33:51 MANUFACTURER -NA-
MAC 50:7c:6f:3e:8d:7d IP 10.255.6.248 HOSTNAME -NA- BEGIN 2023-09-03 08:13:31 END 2023-09-03 16:54:29 MANUFACTURER -NA-
MAC 50:7c:6f:3e:91:b7 IP 10.255.6.208 HOSTNAME -NA- BEGIN 2023-09-03 17:50:58 END 2023-09-03 17:52:58 MANUFACTURER -NA-
MAC 50:7c:6f:3e:90:63 IP 10.255.6.140 HOSTNAME -NA- BEGIN 2023-09-03 18:03:04 END 2023-09-03 18:05:04 MANUFACTURER -NA-
MAC 50:7c:6f:3e:91:c5 IP 10.255.6.192 HOSTNAME -NA- BEGIN 2023-09-03 20:45:03 END 2023-09-03 20:47:03 MANUFACTURER -NA-
`
const parsableLeasesDateFormat = "2006-01-02 15:04:05"

func TestParseLeaseListOutput(t *testing.T) {
	var leases Leases
	for _, line := range strings.Split(leaseListOutput, "\n") {
		if !strings.HasPrefix(line, "MAC") ||
			!strings.Contains(line, "HOSTNAME") ||
			!strings.Contains(line, "BEGIN") ||
			!strings.Contains(line, "END") {
			continue
		}
		// MAC 50:7c:6f:3e:91:c5 IP 10.255.6.192 HOSTNAME -NA- BEGIN 2023-09-03 20:45:03 END 2023-09-03 20:47:03 MANUFACTURER -NA-
		fields := strings.Fields(line)
		if len(fields) < 12 {
			continue
		}
		mac := fields[1]
		ip := fields[3]
		begin, err := time.Parse(parsableLeasesDateFormat, fields[7]+" "+fields[8])
		require.NoError(t, err)

		end, err := time.Parse(parsableLeasesDateFormat, fields[10]+" "+fields[11])
		require.NoError(t, err)
		l := Lease{
			Mac:   mac,
			Ip:    ip,
			Begin: begin,
			End:   end,
		}
		leases = append(leases, l)
	}

	t.Logf("Leases:\n%#v\n", leases)
}

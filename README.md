# metal-bmc

`metal-bmc` is responsible to gather data from machines via the out of band interfaces and report them back to the metal-api.
It also passes commands to the machines like power on/off, led on/off, firmware update etc.
Access to the console of a machine is also terminated here in conjunction with the `metal-console` running in the control-plane.

More details per package as follows:

## Reporter

Reporter reports the ip addresses that are leased to ipmi devices together with their machine uuids to the `metal-api`.
Therewith it is possible to have knowledge about new machines very early in the `metal-api` and also get knowledge about possibly changing ipmi ip addresses.
`metal-bmc` parses the DHCPD lease file and reports the mapping of machine uuids to ipmi ip address to the `metal-api`.

### Lease file formats

Both the `isc-dhcp-server` and the `kea-dhcp-server` lease file formats are supported. Which lease file is read and how it is parsed is configured with two environment variables:

| Variable                  | Default                     | Description                        |
| ------------------------- | --------------------------- | ---------------------------------- |
| `METAL_BMC_LEASE_FILE`    | `/var/lib/dhcp/dhcpd.leases` | the lease file to read             |
| `METAL_BMC_LEASE_FORMAT`  | `isc`                        | either `isc` or `kea`              |

For the `kea-dhcp-server` the lease file is the memfile csv, which is usually located at `/var/lib/kea/kea-leases4.csv`:

```
METAL_BMC_LEASE_FILE=/var/lib/kea/kea-leases4.csv
METAL_BMC_LEASE_FORMAT=kea
```

The memfile is written append-only, so the same address can occur several times, whereby the last entry wins. Leases that were removed by `kea` (valid lifetime of zero) and leases that are not in the default state (declined, expired-reclaimed, released) are not reported.

## BMC

The `bmc` package serves the following:

### Commands

Commands from the metal-api are passed via nsq and executed either through redfish or ipmi against the out-of-band interface of a machine.

### Firmware

Firmware updates the firmware of the BIOS and the BMC of a machine.

### Console

Console forwards the the serial console access terminated in `metal-console` to the machine.
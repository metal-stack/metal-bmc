# bmc-catcher

Reports the ip addresses that are leased to ipmi devices together with their machine uuids to the `metal-api`.

Therewith it is possible to have knowledge about new machines very early in the `metal-api` and also get knowledge about possibly changing ipmi ip addresses.

`bmc-catcher` parses the DHCPD lease file and reports the mapping of machine uuids to ipmi ip address to the `metal-api`.

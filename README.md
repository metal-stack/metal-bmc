# ipmi-catcher
[![FOSSA Status](https://app.fossa.io/api/projects/git%2Bgithub.com%2Fmetal-stack%2Fipmi-catcher.svg?type=shield)](https://app.fossa.io/projects/git%2Bgithub.com%2Fmetal-stack%2Fipmi-catcher?ref=badge_shield)


Reports the ip addresses that are leased to ipmi devices together with their machine uuids to the `metal-api`.

Therewith it is possible to have knowledge about new machines very early in the `metal-api` and also get knowledge about possibly changing ipmi ip addresses.

`ipmi-catcher` parses the DHCPD lease file and reports the mapping of machine uuids to ipmi ip address to the `metal-api`.


## License
[![FOSSA Status](https://app.fossa.io/api/projects/git%2Bgithub.com%2Fmetal-stack%2Fipmi-catcher.svg?type=large)](https://app.fossa.io/projects/git%2Bgithub.com%2Fmetal-stack%2Fipmi-catcher?ref=badge_large)
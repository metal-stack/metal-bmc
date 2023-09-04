package leases

import (
	"errors"
	"regexp"
	"time"
)

const DATE_FORMAT = "2006/01/02 15:04:05"
const LEASE_REGEX = `(?ms)lease\s+(?P<ip>[^\s]+)\s+{.*?starts\s\d+\s(?P<begin>[\d\/]+\s[\d\:]+);.*?ends\s\d+\s(?P<end>[\d\/]+\s[\d\:]+);.*?hardware\sethernet\s(?P<mac>[\w\:]+);.*?}`

func parse(contents string) (Leases, error) {
	leases := Leases{}
	var re = regexp.MustCompile(LEASE_REGEX)
	matches := re.FindAllStringSubmatch(contents, -1)
	var errs []error
	for _, m := range matches {
		rm := make(map[string]string)
		for i, name := range re.SubexpNames() {
			if i != 0 && name != "" {
				rm[name] = m[i]
			}
		}
		begin, err := time.Parse(DATE_FORMAT, rm["begin"])
		if err != nil {
			errs = append(errs, err)
		}
		end, err := time.Parse(DATE_FORMAT, rm["end"])
		if err != nil {
			errs = append(errs, err)
		}

		l := Lease{
			Mac:   rm["mac"],
			Ip:    rm["ip"],
			Begin: begin,
			End:   end,
		}
		leases = append(leases, l)
	}
	if len(errs) > 0 {
		return leases, errors.Join(errs...)
	}
	return leases, nil
}

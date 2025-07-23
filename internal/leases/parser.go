package leases

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

func parse(r io.Reader) (Leases, error) {
	reader := csv.NewReader(r)

	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read header: %w", err)
	}

	if len(header) < 5 || header[0] != "address" || header[1] != "hwaddr" {
		return nil, fmt.Errorf("invalid Kea lease file format")
	}

	var leases Leases
	var errs []error

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			line, _ := reader.FieldPos(0)
			errs = append(errs, fmt.Errorf("line %d: failed to read CSV record: %v", line, err))
			continue
		}

		if len(record) < 5 {
			line, _ := reader.FieldPos(0)
			errs = append(errs, fmt.Errorf("line %d: incomplete record, expected at least 5 fields, got %d", line, len(record)))
			continue
		}

		expireStr := strings.TrimSpace(record[4])
		expireTs, err := strconv.ParseInt(expireStr, 10, 64)
		if err != nil {
			line, col := reader.FieldPos(4)
			errs = append(errs, fmt.Errorf("line %d, column %d: invalid expire timestamp '%s': %v", line, col, expireStr, err))
			continue
		}

		ip := strings.TrimSpace(record[0])
		if ip == "" {
			line, col := reader.FieldPos(0)
			errs = append(errs, fmt.Errorf("line %d, column %d: empty Ip address", line, col))
			continue
		}

		mac := strings.TrimSpace(record[1])
		if mac == "" {
			line, col := reader.FieldPos(1)
			errs = append(errs, fmt.Errorf("line %d, column %d: empty Mac address", line, col))
			continue
		}

		lease := Lease{
			Mac: mac,
			Ip:  ip,
			End: time.Unix(expireTs, 0),
		}
		leases = append(leases, lease)
	}

	if len(errs) > 0 {
		return leases, errors.Join(errs...)
	}

	return leases, nil
}

package bmc

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"testing"
)

func TestCertAuthorizedFor(t *testing.T) {
	mkCert := func(cn string, sans ...string) *x509.Certificate {
		return &x509.Certificate{
			Subject:   pkix.Name{CommonName: cn},
			DNSNames:  sans,
		}
	}

	tests := []struct {
		name      string
		cert      *x509.Certificate
		machineID string
		want      bool
	}{
		{
			name:      "CN matches machineID",
			cert:      mkCert("machine-1"),
			machineID: "machine-1",
			want:      true,
		},
		{
			name:      "DNS SAN matches machineID",
			cert:      mkCert("operator@example", "machine-2", "machine-3"),
			machineID: "machine-3",
			want:      true,
		},
		{
			name:      "CN mismatch and no SAN rejects",
			cert:      mkCert("machine-1"),
			machineID: "machine-2",
			want:      false,
		},
		{
			name:      "SAN mismatch rejects",
			cert:      mkCert("operator@example", "machine-2"),
			machineID: "machine-1",
			want:      false,
		},
		{
			name:      "empty machineID never matches",
			cert:      mkCert("", ""),
			machineID: "",
			want:      false,
		},
		{
			name:      "nil cert never matches",
			cert:      nil,
			machineID: "machine-1",
			want:      false,
		},
		{
			name:      "empty CN does not match empty machineID",
			cert:      mkCert(""),
			machineID: "",
			want:      false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := certAuthorizedFor(tc.cert, tc.machineID)
			if got != tc.want {
				t.Fatalf("certAuthorizedFor(%+v, %q) = %v, want %v", tc.cert, tc.machineID, got, tc.want)
			}
		})
	}
}

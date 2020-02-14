package reporter

import (
	"fmt"
	"net/url"
	"time"

	"github.com/go-openapi/runtime"
	httptransport "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
	"github.com/metal-stack/ipmi-catcher/metal-api/client/machine"
	"github.com/metal-stack/security"
)

type driver struct {
	machine *machine.Client
	auth    runtime.ClientAuthInfoWriter
	hmac    *security.HMACAuth
}

func newDriver(rawurl, hmac string) (*driver, error) {
	parsedurl, err := url.Parse(rawurl)
	if err != nil {
		return nil, err
	}
	if parsedurl.Host == "" {
		return nil, fmt.Errorf("invalid url:%s, must be in the form scheme://host[:port]/basepath", rawurl)
	}
	transport := httptransport.New(parsedurl.Host, parsedurl.Path, []string{parsedurl.Scheme})
	driver := &driver{
		machine: machine.New(transport, strfmt.Default),
	}
	if hmac != "" {
		auth := security.NewHMACAuth("Metal-Edit", []byte(hmac))
		driver.hmac = &auth
	}
	driver.auth = runtime.ClientAuthInfoWriterFunc(driver.auther)
	return driver, nil
}

func (d *driver) auther(rq runtime.ClientRequest, rg strfmt.Registry) error {
	if d.hmac != nil {
		d.hmac.AddAuthToClientRequest(rq, time.Now())
	}
	return nil
}

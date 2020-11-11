package uuid

import (
	"github.com/metal-stack/go-hal/connect"
	halzap "github.com/metal-stack/go-hal/pkg/logger/zap"

	"github.com/pkg/errors"

	"go.uber.org/zap"
)

type Loader struct {
	ipmiPort     int
	ipmiUser     string
	ipmiPassword string
	log          *zap.SugaredLogger
}

func New(ipmiPort int, ipmiUser, ipmiPassword string) *Loader {
	z, _ := zap.NewProduction()
	return &Loader{
		ipmiPort:     ipmiPort,
		ipmiUser:     ipmiUser,
		ipmiPassword: ipmiPassword,
		log:          z.Sugar(),
	}
}

func (u *Loader) LoadFrom(ip string) (string, error) {
	ob, err := connect.OutBand(ip, u.ipmiPort, u.ipmiUser, u.ipmiPassword, halzap.New(u.log))
	if err != nil {
		return "", errors.Wrapf(err, "could not open out-band connection to ip:%s, port:%d, user: %s, error: %v", ip, u.ipmiPort, u.ipmiUser, err)
	}

	uuid, err := ob.UUID()
	if err != nil {
		return "", errors.Wrapf(err, "failed to load UUID from ip:%s", ip)
	}

	return uuid.String(), nil
}

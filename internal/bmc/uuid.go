package bmc

import (
	"github.com/metal-stack/go-hal/connect"
	"github.com/pkg/errors"

	"go.uber.org/zap"
)

type UUIDLoader struct {
	ipmiPort     int
	ipmiUser     string
	ipmiPassword string
	log          *zap.SugaredLogger
}

func NewUUIDLoader(ipmiPort int, ipmiUser, ipmiPassword string) *UUIDLoader {
	z, _ := zap.NewProduction()
	return &UUIDLoader{
		ipmiPort:     ipmiPort,
		ipmiUser:     ipmiUser,
		ipmiPassword: ipmiPassword,
		log:          z.Sugar(),
	}
}

func (u *UUIDLoader) LoadFrom(ip string) (string, error) {
	ob, err := connect.OutBand(ip, u.ipmiPort, u.ipmiUser, u.ipmiPassword)
	if err != nil {
		return "", errors.Wrapf(err, "could not open out-band connection to ip:%s, port:%d, user: %s, error: %v", ip, u.ipmiPort, u.ipmiUser, err)
	}

	uuid, err := ob.UUID()
	if err != nil {
		return "", errors.Wrapf(err, "failed to load UUID from ip:%s", ip)
	}

	return uuid.String(), nil
}

package bmc

import (
	"github.com/metal-stack/go-hal/connect"
	"github.com/pkg/errors"
	"sync"

	"go.uber.org/zap"
)

type UUIDCache struct {
	macToUUID    map[string]string
	ipmiPort     int
	ipmiUser     string
	ipmiPassword string
	log          *zap.SugaredLogger
}

type entry struct {
	mac  string
	uuid string
}

func NewUUIDCache(ipmiPort int, ipmiUser, ipmiPassword string) UUIDCache {
	z, _ := zap.NewProduction()
	return UUIDCache{
		macToUUID:    map[string]string{},
		ipmiPort:     ipmiPort,
		ipmiUser:     ipmiUser,
		ipmiPassword: ipmiPassword,
		log:          z.Sugar(),
	}
}

// Warmup fetches uuids of given ips
func (u UUIDCache) Warmup(macToIps map[string]string) {
	var wg sync.WaitGroup
	ch := make(chan entry)
	for mac, ip := range macToIps {
		wg.Add(1)
		go u.warmupWorker(&wg, mac, ip, ch)
	}
	go func() {
		wg.Wait()
		close(ch)
	}()
	for e := range ch {
		u.macToUUID[e.mac] = e.uuid
	}
}

func (u UUIDCache) warmupWorker(wg *sync.WaitGroup, mac, ip string, ch chan entry) {
	defer wg.Done()
	uuid, err := u.loadUUID(ip, u.ipmiPort, u.ipmiUser, u.ipmiPassword)
	if err != nil {
		u.log.Errorw("warmupWorker", "error during loadUUID", err)
		return
	}
	ch <- entry{
		uuid: uuid,
		mac:  mac,
	}
}

// Get lazy fetch a machine uuid from a warm cache, if not present fetch it.
func (u UUIDCache) Get(mac, ip string) (*string, error) {
	if uuid, ok := u.macToUUID[mac]; ok {
		return &uuid, nil
	}
	uuid, err := u.loadUUID(ip, u.ipmiPort, u.ipmiUser, u.ipmiPassword)
	if err != nil {
		return nil, err
	}
	u.macToUUID[mac] = uuid
	return &uuid, nil
}

func (u UUIDCache) loadUUID(ip string, port int, user, password string) (string, error) {
	ob, err := connect.OutBand(ip, port, user, password)
	if err != nil {
		return "", errors.Wrapf(err, "could not open out-band connection to ip:%s, port:%d, user: %s, error: %v", ip, port, user, err)
	}

	uuid, err := ob.UUID()
	if err != nil {
		return "", errors.Wrapf(err, "failed to load UUID from ip:%s", ip)
	}

	return string(uuid[:]), nil
}

package ipmi

import (
	"bufio"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"sync"

	"go.uber.org/zap"
)

const (
	uuidRegex = `([0-9A-F]{8}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{12})`
)

var (
	uuidRegexCompiled = regexp.MustCompile(uuidRegex)
)

type UUIDCache struct {
	macToUUID    map[string]string
	ipmiUser     string
	ipmiPassword string
	sumBin       string
	log          *zap.SugaredLogger
}

type entry struct {
	mac  string
	uuid string
}

func NewUUIDCache(ipmiUser, ipmiPassword, sumBin string) UUIDCache {
	z, _ := zap.NewProduction()
	return UUIDCache{
		macToUUID:    map[string]string{},
		ipmiUser:     ipmiUser,
		ipmiPassword: ipmiPassword,
		sumBin:       sumBin,
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
	uuid, err := u.loadUUID(ip, u.ipmiUser, u.ipmiPassword, u.sumBin)
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
	uuid, err := u.loadUUID(ip, u.ipmiUser, u.ipmiPassword, u.sumBin)
	if err != nil {
		return nil, err
	}
	u.macToUUID[mac] = uuid
	return &uuid, nil
}

func parseUUIDLine(l string) string {
	return strings.ToLower(uuidRegexCompiled.FindString(l))
}

func (u UUIDCache) loadUUID(ip, user, password, sum string) (string, error) {
	args := []string{"--no_banner", "--no_progress", "--journal_level", "0", "-i", ip, "-u", user, "-p", password, "-c", "GetDmiInfo"}
	cmd := exec.Command(sum, args...)
	out, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("could not initiate sum command to get dmi data from ip:%s, err: %v", ip, err)
	}
	err = cmd.Start()
	if err != nil {
		return "", fmt.Errorf("could not start sum command to get dmi data from ip:%s, err: %v", ip, err)
	}
	go func() {
		err = cmd.Wait()
		if err != nil {
			u.log.Errorw("loadUUID", "ip", ip, "wait error", err)
		}
	}()
	s := bufio.NewScanner(out)
	for s.Scan() {
		l := s.Text()
		if strings.HasPrefix(l, "UUID") {
			return parseUUIDLine(l), nil
		}
	}
	return "", fmt.Errorf("could not find UUID in dmi data for ip:%s", ip)
}

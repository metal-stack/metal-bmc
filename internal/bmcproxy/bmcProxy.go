package bmcproxy

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/metal-stack/go-hal/connect"
	halzap "github.com/metal-stack/go-hal/pkg/logger/zap"

	"github.com/gliderlabs/ssh"
	"github.com/metal-stack/metal-go/api/models"
	"go.uber.org/zap"
	gossh "golang.org/x/crypto/ssh"
)

type bmcProxy struct {
	log  *zap.SugaredLogger
	port int
}

func New(log *zap.SugaredLogger, port int) *bmcProxy {
	return &bmcProxy{
		log:  log,
		port: port,
	}
}

// Run starts ssh server and listen for console connections.
func (p *bmcProxy) Run() {
	s := &ssh.Server{
		Addr:    fmt.Sprintf(":%d", p.port),
		Handler: p.sessionHandler,
	}

	hostKey, err := loadHostKey()
	if err != nil {
		p.log.Errorw("cannot load host key", "error", err)
		os.Exit(1)
	}
	s.AddHostKey(hostKey)

	p.log.Infow("starting ssh server", "port", p.port)
	p.log.Fatal(s.ListenAndServe())
}

func (p *bmcProxy) sessionHandler(s ssh.Session) {
	p.log.Infow("ssh session handler called", "user", s.User(), "env", s.Environ())
	machineID := s.User()
	metalIPMI := p.receiveIPMIData(s)
	p.log.Infow("connection to", "machineID", machineID)
	if metalIPMI == nil {
		p.log.Errorw("failed to receive IPMI data", "machineID", machineID)
		return
	}
	if metalIPMI.Address == nil {
		p.log.Errorw("failed to receive IPMI.Address data", "machineID", machineID)
		return
	}
	_, err := io.WriteString(s, fmt.Sprintf("Connecting to console of %q (%s)\n", machineID, *metalIPMI.Address))
	if err != nil {
		p.log.Warnw("failed to write to console", "machineID", machineID)
	}

	host, portStr, found := strings.Cut(*metalIPMI.Address, ":")
	if !found {
		p.log.Errorw("invalid ipmi address", "address", *metalIPMI.Address)
		return
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		p.log.Errorw("invalid port", "port", port, "address", *metalIPMI.Address)
		return
	}

	ob, err := connect.OutBand(host, port, *metalIPMI.User, *metalIPMI.Password, halzap.New(p.log))
	if err != nil {
		p.log.Errorw("failed to out-band connect", "host", host, "port", port, "machineID", machineID, "ipmiuser", *metalIPMI.User)
		return
	}

	err = ob.Console(s)
	if err != nil {
		p.log.Errorw("failed to access console", "machineID", machineID, "error", err)
	}
}

func (p *bmcProxy) receiveIPMIData(s ssh.Session) *models.V1MachineIPMI {
	var ipmiData string
	for i := 0; i < 5; i++ {
		for _, env := range s.Environ() {
			_, data, found := strings.Cut(env, "LC_IPMI_DATA=")
			if found {
				ipmiData = data
				break
			}
		}
		if len(ipmiData) > 0 {
			break
		}
		time.Sleep(1 * time.Second)
	}

	if len(ipmiData) == 0 {
		p.log.Error("failed to receive IPMI data")
		return nil
	}

	metalIPMI := &models.V1MachineIPMI{}
	err := metalIPMI.UnmarshalBinary([]byte(ipmiData))
	if err != nil {
		p.log.Errorw("failed to unmarshal received IPMI data", "error", err)
		return nil
	}

	return metalIPMI
}

func loadHostKey() (gossh.Signer, error) {
	bb, err := os.ReadFile("/server-key.pem")
	if err != nil {
		return nil, fmt.Errorf("failed to load private key:%w", err)
	}
	return gossh.ParsePrivateKey(bb)
}

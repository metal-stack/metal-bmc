package bmcconsole

import (
	"crypto/tls"
	"crypto/x509"
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

type bmcConsole struct {
	log       *zap.SugaredLogger
	tlsConfig *tls.Config
	port      int
	hostKey   gossh.Signer
}

func New(log *zap.SugaredLogger, caCertFile, certFile, keyFile string, port int) (*bmcConsole, error) {

	caCert, err := os.ReadFile(caCertFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load cert: %w", err)
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},        // server certificate which is validated by the client
		ClientCAs:    caCertPool,                     // used to verify the client cert is signed by the CA and is therefore valid
		ClientAuth:   tls.RequireAndVerifyClientCert, // this requires a valid client certificate to be supplied during handshake
		MinVersion:   tls.VersionTLS13,
	}

	hostKey, err := loadHostKey()
	if err != nil {
		return nil, fmt.Errorf("cannot load host key %w", err)
	}
	return &bmcConsole{
		log:       log,
		tlsConfig: tlsConfig,
		port:      port,
		hostKey:   hostKey,
	}, nil
}

// ListenAndServe starts ssh server and listen for console connections.
func (c *bmcConsole) ListenAndServe() error {
	s := &ssh.Server{
		Handler: c.sessionHandler,
	}
	s.AddHostKey(c.hostKey)
	listener, err := tls.Listen("tcp", fmt.Sprintf(":%d", c.port), c.tlsConfig)
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}
	c.log.Infow("starting ssh server", "address", s.Addr)
	return s.Serve(listener)
}

// FIXME broken error handling, should also be printed to the session
func (c *bmcConsole) sessionHandler(s ssh.Session) {
	c.log.Infow("ssh session handler called", "user", s.User(), "env", s.Environ())
	machineID := s.User()
	metalIPMI := c.receiveIPMIData(s)
	c.log.Infow("connection to", "machineID", machineID)
	if metalIPMI == nil {
		c.log.Errorw("failed to receive IPMI data", "machineID", machineID)
		return
	}
	if metalIPMI.Address == nil {
		c.log.Errorw("failed to receive IPMI.Address data", "machineID", machineID)
		return
	}
	_, err := io.WriteString(s, fmt.Sprintf("Connecting to console of %q (%s)\n", machineID, *metalIPMI.Address))
	if err != nil {
		c.log.Warnw("failed to write to console", "machineID", machineID)
	}

	host, portStr, found := strings.Cut(*metalIPMI.Address, ":")
	if !found {
		c.log.Errorw("invalid ipmi address", "address", *metalIPMI.Address)
		return
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		c.log.Errorw("invalid port", "port", port, "address", *metalIPMI.Address)
		return
	}

	ob, err := connect.OutBand(host, port, *metalIPMI.User, *metalIPMI.Password, halzap.New(c.log))
	if err != nil {
		c.log.Errorw("failed to out-band connect", "host", host, "port", port, "machineID", machineID, "ipmiuser", *metalIPMI.User)
		return
	}

	err = ob.Console(s)
	if err != nil {
		c.log.Errorw("failed to access console", "machineID", machineID, "error", err)
	}
}

// FIXME should return an error
func (c *bmcConsole) receiveIPMIData(s ssh.Session) *models.V1MachineIPMI {
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
		c.log.Error("failed to receive IPMI data")
		return nil
	}

	metalIPMI := &models.V1MachineIPMI{}
	err := metalIPMI.UnmarshalBinary([]byte(ipmiData))
	if err != nil {
		c.log.Errorw("failed to unmarshal received IPMI data", "error", err)
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

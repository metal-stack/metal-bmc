package bmc

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/metal-stack/go-hal/connect"
	halzap "github.com/metal-stack/go-hal/pkg/logger/zap"
	metalgo "github.com/metal-stack/metal-go"
	"github.com/metal-stack/metal-go/api/client/machine"

	"github.com/gliderlabs/ssh"
	"go.uber.org/zap"
	gossh "golang.org/x/crypto/ssh"
)

type console struct {
	log       *zap.SugaredLogger
	tlsConfig *tls.Config
	port      int
	hostKey   gossh.Signer
	client    metalgo.Client
}

func NewConsole(log *zap.SugaredLogger, client metalgo.Client, caCertFile, certFile, keyFile string, port int) (*console, error) {

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

	bb, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load ssh server key:%w", err)
	}
	hostKey, err := gossh.ParsePrivateKey(bb)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ssh server key:%w", err)
	}

	return &console{
		log:       log,
		tlsConfig: tlsConfig,
		port:      port,
		hostKey:   hostKey,
		client:    client,
	}, nil
}

// ListenAndServe starts ssh server and listen for console connections.
func (c *console) ListenAndServe() error {
	s := &ssh.Server{
		Handler: c.sessionHandler,
	}
	s.AddHostKey(c.hostKey)
	addr := fmt.Sprintf(":%d", c.port)
	listener, err := tls.Listen("tcp", addr, c.tlsConfig)
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}
	c.log.Infow("starting ssh server", "address", addr)
	return s.Serve(listener)
}

// FIXME broken error handling, should also be printed to the session
func (c *console) sessionHandler(s ssh.Session) {
	c.log.Infow("ssh session handler called", "user", s.User(), "env", s.Environ())
	machineID := s.User()

	resp, err := c.client.Machine().FindIPMIMachine(machine.NewFindIPMIMachineParams().WithID(machineID), nil)
	if err != nil || resp.Payload == nil || resp.Payload.Ipmi == nil {
		c.log.Errorw("failed to receive IPMI data", "machineID", machineID, "error", err)
		return
	}
	metalIPMI := resp.Payload.Ipmi

	c.log.Infow("connection to", "machineID", machineID)
	if metalIPMI == nil {
		c.log.Errorw("failed to receive IPMI data", "machineID", machineID)
		return
	}
	if metalIPMI.Address == nil {
		c.log.Errorw("failed to receive IPMI.Address data", "machineID", machineID)
		return
	}
	_, err = io.WriteString(s, fmt.Sprintf("Connecting to console of %q (%s)\n", machineID, *metalIPMI.Address))
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

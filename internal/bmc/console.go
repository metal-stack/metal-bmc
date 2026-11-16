package bmc

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/metal-stack/go-hal/connect"
	halslog "github.com/metal-stack/go-hal/pkg/logger/slog"
	"github.com/metal-stack/metal-bmc/pkg/config"
	metalgo "github.com/metal-stack/metal-go"
	"github.com/metal-stack/metal-go/api/client/machine"

	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"
)

type console struct {
	log       *slog.Logger
	tlsConfig *tls.Config
	port      int
	hostKey   gossh.Signer
	client    metalgo.Client
}

func NewConsole(log *slog.Logger, client metalgo.Client, c config.Config) (*console, error) {

	caCert, err := os.ReadFile(c.ConsoleCACertFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load cert: %w", err)
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	cert, err := tls.LoadX509KeyPair(c.ConsoleCertFile, c.ConsoleKeyFile)
	if err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},        // server certificate which is validated by the client
		ClientCAs:    caCertPool,                     // used to verify the client cert is signed by the CA and is therefore valid
		ClientAuth:   tls.RequireAndVerifyClientCert, // this requires a valid client certificate to be supplied during handshake
		MinVersion:   tls.VersionTLS13,
	}

	bb, err := os.ReadFile(c.ConsoleKeyFile)
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
		port:      c.ConsolePort,
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
	c.log.Info("starting ssh server", "address", addr)
	return s.Serve(listener)
}

func (c *console) sessionHandler(s ssh.Session) {
	machineID := s.User()
	log := c.log.With("machineID", machineID)

	log.Info("ssh session handler called")

	sessionError := func(msg string, err error, attrs ...any) {
		log.Error(msg, append(attrs, "error", err)...)

		if _, werr := fmt.Fprintf(s, "\r\n%s: %v\r\n", msg, err); werr != nil {
			c.log.Warn("failed to write error to console", "machineID", machineID, "error", werr)
		}
	}

	resp, err := c.client.Machine().FindIPMIMachine(machine.NewFindIPMIMachineParams().WithID(machineID), nil)
	if err != nil {
		sessionError("failed to receive IPMI data", err)
		return
	}
	if resp.Payload == nil || resp.Payload.Ipmi == nil {
		sessionError("failed to receive IPMI data", errors.New("no IPMI data"))
		return
	}

	metalIPMI := resp.Payload.Ipmi

	c.log.Info("connection to", "machineID", machineID)
	if metalIPMI.Address == nil {
		sessionError("failed to receive IPMI address", errors.New("no IPMI address"))
		return
	}
	_, err = io.WriteString(s, fmt.Sprintf("Connecting to console of %q (%s)\n", machineID, *metalIPMI.Address))
	if err != nil {
		c.log.Warn("failed to write to console", "machineID", machineID)
	}

	host, portStr, found := strings.Cut(*metalIPMI.Address, ":")
	if !found {
		sessionError("invalid IPMI address", fmt.Errorf("%q", *metalIPMI.Address))
		return
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		sessionError("invalid IPMI port", err)
		return
	}

	ob, err := connect.OutBand(host, port, *metalIPMI.User, *metalIPMI.Password, halslog.New(c.log), new(time.Minute))
	if err != nil {
		sessionError("failed to out-band connect", err, "host", host, "port", port, "ipmiuser", *metalIPMI.User)
		return
	}

	err = ob.Console(s)
	if err != nil {
		if errors.Is(err, io.EOF) {
			c.log.Info("console access terminated")
		} else {
			sessionError("failed to access console", err)
		}
	}
}

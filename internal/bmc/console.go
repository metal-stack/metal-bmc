package bmc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"

	apiclient "github.com/metal-stack/api/go/client"
	adminv2 "github.com/metal-stack/api/go/metalstack/admin/v2"
	halconnect "github.com/metal-stack/go-hal/connect"
	halslog "github.com/metal-stack/go-hal/pkg/logger/slog"
	"github.com/metal-stack/metal-bmc/pkg/config"

	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"
)

type console struct {
	log       *slog.Logger
	tlsConfig *tls.Config
	port      int
	hostKey   gossh.Signer
	client    apiclient.Client
}

func NewConsole(log *slog.Logger, client apiclient.Client, c config.Config) (*console, error) {

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

// FIXME broken error handling, should also be printed to the session
func (c *console) sessionHandler(s ssh.Session) {
	c.log.Info("ssh session handler called", "machineID", s.User())
	machineID := s.User()

	resp, err := c.client.Adminv2().Machine().GetBMC(context.Background(), &adminv2.MachineServiceGetBMCRequest{Uuid: machineID})
	if err != nil {
		c.log.Error("failed to receive BMC data", "machineID", machineID, "error", err)
		return
	}

	bmc := resp.Bmc.Bmc
	if bmc.Address == "" {
		c.log.Error("failed to receive BMC.Address data", "machineID", machineID)
		return
	}
	_, err = io.WriteString(s, fmt.Sprintf("Connecting to console of %q (%s)\n", machineID, bmc.Address))
	if err != nil {
		c.log.Warn("failed to write to console", "machineID", machineID)
	}

	host, portStr, found := strings.Cut(bmc.Address, ":")
	if !found {
		c.log.Error("invalid ipmi address", "address", bmc.Address)
		return
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		c.log.Error("invalid port", "port", port, "address", bmc.Address)
		return
	}

	ob, err := halconnect.OutBand(host, port, bmc.User, bmc.Password, halslog.New(c.log))
	if err != nil {
		c.log.Error("failed to out-band connect", "host", host, "port", port, "machineID", machineID, "ipmiuser", bmc.User)
		return
	}

	err = ob.Console(s)
	if err != nil {
		if errors.Is(err, io.EOF) {
			c.log.Info("console access terminated")
		} else {
			c.log.Error("failed to access console", "machineID", machineID, "error", err)
		}
	}
}

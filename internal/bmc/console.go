package bmc

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"slices"
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

// tlsPeerCertContextKey is the ssh.Context key under which the verified TLS
// peer certificate is stashed by the ConnCallback. Using a pointer to a local
// unexported type follows the gliderlabs/ssh convention for context keys.
var tlsPeerCertContextKey = &struct{ name string }{name: "tls-peer-cert"}

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
//
// Trust model: every connection is wrapped in mTLS
// (tls.RequireAndVerifyClientCert). The verified TLS peer certificate — not
// the SSH username — is the only identity we trust. The SSH layer therefore
// intentionally has no PublicKey/Password/KeyboardInteractive handler; the
// gliderlabs/ssh server sets NoClientAuth=true in that case. Authorization
// is performed in sessionHandler by matching the requested machineID against
// the peer certificate (CN / DNS SAN).
func (c *console) ListenAndServe() error {
	s := &ssh.Server{
		Handler:      c.sessionHandler,
		ConnCallback: c.captureTLSPeerCert,
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

// captureTLSPeerCert forces the TLS handshake to complete before any SSH
// bytes flow and stashes the verified client leaf certificate on the SSH
// context. Returning nil causes the connection to be closed.
func (c *console) captureTLSPeerCert(ctx ssh.Context, conn net.Conn) net.Conn {
	tc, ok := conn.(*tls.Conn)
	if !ok {
		c.log.Warn("refusing non-TLS connection to console server",
			"remote", conn.RemoteAddr().String())
		return nil
	}
	if err := tc.HandshakeContext(ctx); err != nil {
		c.log.Warn("TLS handshake failed",
			"remote", conn.RemoteAddr().String(), "error", err)
		return nil
	}
	certs := tc.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		// Should be unreachable because tlsConfig uses
		// RequireAndVerifyClientCert, but fail closed anyway.
		c.log.Warn("no TLS peer certificate after handshake; rejecting",
			"remote", conn.RemoteAddr().String())
		return nil
	}
	ctx.SetValue(tlsPeerCertContextKey, certs[0])
	return conn
}

// certAuthorizedFor reports whether the leaf client certificate is authorized
// to open a console session for machineID. A machineID matches if it equals
// the certificate's Subject CommonName or appears as a DNS SAN. This assumes
// the console CA issues per-machine (or machine-listing) client certificates.
func certAuthorizedFor(cert *x509.Certificate, machineID string) bool {
	if cert == nil || machineID == "" {
		return false
	}
	if cert.Subject.CommonName == machineID {
		return true
	}
	return slices.Contains(cert.DNSNames, machineID)
}

// FIXME broken error handling, should also be printed to the session
func (c *console) sessionHandler(s ssh.Session) {
	machineID := s.User()
	c.log.Info("ssh session handler called", "machineID", machineID)

	cert, _ := s.Context().Value(tlsPeerCertContextKey).(*x509.Certificate)
	if cert == nil {
		c.log.Warn("refusing session without TLS peer certificate", "machineID", machineID)
		_, _ = io.WriteString(s, "unauthorized: client certificate required\n")
		_ = s.Exit(1)
		return
	}
	if !certAuthorizedFor(cert, machineID) {
		c.log.Warn("unauthorized console access attempt",
			"machineID", machineID,
			"cert_subject", cert.Subject.String(),
			"cert_dns_sans", cert.DNSNames,
			"cert_serial", cert.SerialNumber.String(),
		)
		_, _ = io.WriteString(s, "unauthorized: client certificate not issued for this machine\n")
		_ = s.Exit(1)
		return
	}
	c.log.Info("authorized console session",
		"machineID", machineID,
		"cert_subject", cert.Subject.CommonName,
		"cert_serial", cert.SerialNumber.String(),
	)

	resp, err := c.client.Machine().FindIPMIMachine(machine.NewFindIPMIMachineParams().WithID(machineID), nil)
	if err != nil || resp.Payload == nil || resp.Payload.Ipmi == nil {
		c.log.Error("failed to receive IPMI data", "machineID", machineID, "error", err)
		return
	}
	metalIPMI := resp.Payload.Ipmi

	c.log.Info("connection to", "machineID", machineID)
	if metalIPMI == nil {
		c.log.Error("failed to receive IPMI data", "machineID", machineID)
		return
	}
	if metalIPMI.Address == nil {
		c.log.Error("failed to receive IPMI.Address data", "machineID", machineID)
		return
	}
	_, err = io.WriteString(s, fmt.Sprintf("Connecting to console of %q (%s)\n", machineID, *metalIPMI.Address))
	if err != nil {
		c.log.Warn("failed to write to console", "machineID", machineID)
	}

	host, portStr, found := strings.Cut(*metalIPMI.Address, ":")
	if !found {
		c.log.Error("invalid ipmi address", "address", *metalIPMI.Address)
		return
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		c.log.Error("invalid port", "port", port, "address", *metalIPMI.Address)
		return
	}

	ob, err := connect.OutBand(host, port, *metalIPMI.User, *metalIPMI.Password, halslog.New(c.log), new(time.Minute))
	if err != nil {
		c.log.Error("failed to out-band connect", "host", host, "port", port, "machineID", machineID, "ipmiuser", *metalIPMI.User)
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

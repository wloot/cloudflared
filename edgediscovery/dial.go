package edgediscovery

import (
	"context"
	"crypto/tls"
	"net"
	"syscall"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

// DialEdgeWithH2Mux makes a TLS connection to a Cloudflare edge node
func DialEdge(
	ctx context.Context,
	timeout time.Duration,
	tlsConfig *tls.Config,
	edgeTCPAddr *net.TCPAddr,
) (net.Conn, error) {
	// Inherit from parent context so we can cancel (Ctrl-C) while dialing
	dialCtx, dialCancel := context.WithTimeout(ctx, timeout)
	defer dialCancel()

	dialer := net.Dialer{
		Control: func(network string, address string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				unix.SetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_NOTSENT_LOWAT, 16384)
			})
		},
	}
	edgeConn, err := dialer.DialContext(dialCtx, "tcp", edgeTCPAddr.String())
	if err != nil {
		return nil, newDialError(err, "DialContext error")
	}

	tlsEdgeConn := tls.Client(edgeConn, tlsConfig)
	tlsEdgeConn.SetDeadline(time.Now().Add(timeout))

	if err = tlsEdgeConn.Handshake(); err != nil {
		return nil, newDialError(err, "TLS handshake with edge error")
	}
	// clear the deadline on the conn; h2mux has its own timeouts
	tlsEdgeConn.SetDeadline(time.Time{})
	return tlsEdgeConn, nil
}

// DialError is an error returned from DialEdge
type DialError struct {
	cause error
}

func newDialError(err error, message string) error {
	return DialError{cause: errors.Wrap(err, message)}
}

func (e DialError) Error() string {
	return e.cause.Error()
}

func (e DialError) Cause() error {
	return e.cause
}

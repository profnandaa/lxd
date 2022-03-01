package tcp

import (
	"crypto/tls"
	"fmt"
	"net"
	"reflect"
	"time"
	"unsafe"
)

// ExtractConn tries to extract the underlying net.TCPConn from a tls.Conn.
func ExtractConn(conn net.Conn) (*net.TCPConn, error) {
	// Go doesn't currently expose the underlying TCP connection of a TLS connection, but we need it in order
	// to set timeout properties on the connection. We use some reflect/unsafe magic to extract the private
	// remote.conn field, which is indeed the underlying TCP connection.
	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		return nil, fmt.Errorf("Connection is not a tls.Conn")
	}

	field := reflect.ValueOf(tlsConn).Elem().FieldByName("conn")
	field = reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem()
	c := field.Interface()

	tcpConn, ok := c.(*net.TCPConn)
	if !ok {
		return nil, fmt.Errorf("Connection is not a net.TCPConn")
	}

	return tcpConn, nil
}

// SetTimeouts sets TCP_USER_TIMEOUT and TCP keep alive timeouts on a connection.
func SetTimeouts(conn *net.TCPConn) error {
	// Set TCP_USER_TIMEOUT option to limit the maximum amount of time in ms that transmitted data may remain
	// unacknowledged before TCP will forcefully close the corresponding connection and return ETIMEDOUT to the
	// application. This combined with the TCP keepalive options on the socket will ensure that should the
	// remote side of the connection disappear abruptly that LXD will detect this and close the socket quickly.
	// Decreasing the user timeouts allows applications to "fail fast" if so desired. Otherwise it may take
	// up to 20 minutes with the current system defaults in a normal WAN environment if there are packets in
	// the send queue that will prevent the keepalive timer from working as the retransmission timers kick in.
	// See https://git.kernel.org/pub/scm/linux/kernel/git/torvalds/linux.git/commit/?id=dca43c75e7e545694a9dd6288553f55c53e2a3a3
	err := SetUserTimeout(conn, time.Second*30)
	if err != nil {
		return err
	}

	err = conn.SetKeepAlive(true)
	if err != nil {
		return err
	}

	err = conn.SetKeepAlivePeriod(3 * time.Second)
	if err != nil {
		return err
	}

	return nil
}

package memconn

import (
	"net"
	"time"
)

// Conn is an in-memory implementation of Golang's "net.Conn" interface.
type Conn struct {
	net.Conn
	rn         int64
	wn         int64
	localAddr  Addr
	remoteAddr Addr
	isRemote   bool
}

// LocalAddr implements the net.Conn LocalAddr method.
func (c Conn) LocalAddr() net.Addr {
	return c.localAddr
}

// RemoteAddr implements the net.Conn RemoteAddr method.
func (c Conn) RemoteAddr() net.Addr {
	return c.remoteAddr
}

// Close implements the net.Conn Close method.
func (c *Conn) Close() error {
	return c.Conn.Close()
}

// Read implements the net.Conn Read method.
func (c *Conn) Read(b []byte) (rn int, failed error) {
	n, err := c.Conn.Read(b)
	if err != nil {
		if e, ok := err.(*net.OpError); ok {
			e.Addr = c.remoteAddr
			e.Source = c.localAddr
			return n, e
		}
		return n, &net.OpError{Op: "read", Net: "pipe", Err: err}
	}
	return n, nil
}

// Write implements the net.Conn Write method.
func (c *Conn) Write(b []byte) (wn int, failed error) {
	n, err := c.Conn.Write(b)
	if err != nil {
		if e, ok := err.(*net.OpError); ok {
			e.Addr = c.remoteAddr
			e.Source = c.localAddr
			return n, e
		}
		return n, &net.OpError{Op: "write", Net: "pipe", Err: err}
	}
	return n, nil
}

// SetReadDeadline implements the net.Conn SetReadDeadline method.
func (c *Conn) SetReadDeadline(t time.Time) error {
	if err := c.Conn.SetReadDeadline(t); err != nil {
		if e, ok := err.(*net.OpError); ok {
			e.Addr = c.localAddr
			e.Source = c.localAddr
			return e
		}
		return err
	}
	return nil
}

// SetWriteDeadline implements the net.Conn SetWriteDeadline method.
func (c *Conn) SetWriteDeadline(t time.Time) error {
	if err := c.Conn.SetWriteDeadline(t); err != nil {
		if e, ok := err.(*net.OpError); ok {
			e.Addr = c.localAddr
			e.Source = c.localAddr
			return e
		}
		return err
	}
	return nil
}

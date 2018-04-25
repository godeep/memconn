package memconn

import (
	"context"
	"errors"
	"net"
)

// listener implements the net.Listener interface.
type listener struct {
	addr Addr
	rcvr chan net.Conn
	done chan struct{}
	rmvd chan struct{}
}

func (l listener) dial(
	ctx context.Context,
	network string,
	laddr, raddr Addr) (net.Conn, error) {

	// Get two, connected net.Conn objects.
	local, remote := Pipe()

	// Wrap the connections with pipeWrapper so:
	//
	//   * Calls to LocalAddr() and RemoteAddr() return the
	//     correct address information
	//   * Errors returns from the internal pipe are checked and
	//     have their internal OpError addr information replaced with
	//     the correct address information.
	//   * A channel can be setup to cause the event of the Listener
	//     closing closes the remoteConn immediately.
	localConn, remoteConn := &Conn{
		Conn:       local,
		localAddr:  laddr,
		remoteAddr: raddr,
	}, &Conn{
		Conn:       remote,
		localAddr:  raddr,
		remoteAddr: laddr,
		isRemote:   true,
	}

	// Start a goroutine that closes the remote side of the connection
	// as soon as the listener's done channel is no longer blocked.
	go func() {
		<-l.done
		remoteConn.Close()
	}()

	// If the provided context is nill then announce a new connection
	// by placing the new remoteConn onto the rcvr channel. An Accept
	// call from this listener will remove the remoteConn from the channel.
	if ctx == nil {
		l.rcvr <- remoteConn
		return localConn, nil
	}

	// Announce a new connection by placing the new remoteConn
	// onto the rcvr channel. An Accept call from this listener will
	// remove the remoteConn from the channel. However, if that does
	// not occur by the time the context times out / is cancelled, then
	// an error is returned.
	select {
	case l.rcvr <- remoteConn:
		return localConn, nil
	case <-ctx.Done():
		localConn.Close()
		remoteConn.Close()
		return nil, &net.OpError{
			Addr:   raddr,
			Source: laddr,
			Net:    network,
			Op:     "dial",
			Err:    ctx.Err(),
		}
	}
}

// Accept implements the net.Listener Accept method.
func (l listener) Accept() (net.Conn, error) {
	select {
	case remoteConn, ok := <-l.rcvr:
		if ok {
			return remoteConn, nil
		}
		return nil, &net.OpError{
			Addr:   l.addr,
			Source: l.addr,
			Net:    l.addr.Network(),
			Err:    errors.New("listener closed"),
		}
	case <-l.done:
		return nil, &net.OpError{
			Addr:   l.addr,
			Source: l.addr,
			Net:    l.addr.Network(),
			Err:    errors.New("listener closed"),
		}
	}
}

// Close implements the net.Listener Close method.
func (l listener) Close() error {
	select {
	case <-l.done:
		// Already closed
	default:
		// Still open, close it
		close(l.done)

		// Wait for the listener to be removed from the provider cache.
		<-l.rmvd
	}
	return nil
}

// Addr implements the net.Listener Addr method.
func (l listener) Addr() net.Addr {
	return l.addr
}

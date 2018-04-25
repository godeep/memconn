package memconn

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

// Provider is used to track named MemConn objects.
type Provider struct {
	memu listenerCache
	nets networkMap
}

type listenerCache struct {
	sync.RWMutex
	cache map[string]listener
}

type networkMap struct {
	sync.RWMutex
	cache map[string]string
}

// MapNetwork enables mapping the network value provided to this Provider's
// Dial and Listen functions from the specified "from" value to the
// specified "to" value.
//
// For example, calling MapNetwork("tcp", "memu") means a subsequent
// Dial("tcp", "address") gets translated to Dial("memu", "address").
//
// Calling MapNetwork("tcp", "") removes any previous translation for
// the "tcp" network.
func (p *Provider) MapNetwork(from, to string) {
	p.nets.Lock()
	defer p.nets.Unlock()
	if p.nets.cache == nil {
		p.nets.cache = map[string]string{}
	}
	if to == "" {
		delete(p.nets.cache, from)
		return
	}
	p.nets.cache[from] = to
}

func (p *Provider) mapNetwork(network string) string {
	p.nets.RLock()
	defer p.nets.RUnlock()
	if to, ok := p.nets.cache[network]; ok {
		return to
	}
	return network
}

// Listen begins listening at address for the specified network.
//
// Known networks are "memu" (memconn unbuffered).
//
// When the specified address is already in use on the specified
// network an error is returned.
//
// When the provided network is unknown the operation defers to
// net.Dial.
func (p *Provider) Listen(network, address string) (net.Listener, error) {
	switch p.mapNetwork(network) {
	case networkMemu:
		return p.ListenMem(network, &Addr{Name: address})
	default:
		return net.Listen(network, address)
	}
}

// ListenMem begins listening at laddr.
//
// Known networks are "memu" (memconn unbuffered).
//
// If laddr is nil then ListenMem listens on "localhost" on the
// specified network.
func (p *Provider) ListenMem(
	network string, laddr *Addr) (net.Listener, error) {

	var listeners *listenerCache

	switch p.mapNetwork(network) {
	case networkMemu:
		// If laddr is not specified then set it to the reserved name
		// "localhost".
		if laddr == nil {
			laddr = &Addr{Name: addrLocalhost}
		}

		// Verify that network is compatible with laddr.
		if laddr.Buffered {
			return nil, &net.OpError{
				Addr:   laddr,
				Source: laddr,
				Net:    network,
				Op:     "listen",
				Err:    errors.New("incompatible network & local address"),
			}
		}

		listeners = &p.memu
	default:
		return nil, &net.OpError{
			Addr:   laddr,
			Source: laddr,
			Net:    network,
			Op:     "listen",
			Err:    errors.New("unknown network"),
		}
	}

	listeners.Lock()
	defer listeners.Unlock()

	if listeners.cache == nil {
		listeners.cache = map[string]listener{}
	}

	if _, ok := listeners.cache[laddr.Name]; ok {
		return nil, &net.OpError{
			Addr:   laddr,
			Source: laddr,
			Net:    network,
			Op:     "listen",
			Err:    errors.New("addr unavailable"),
		}
	}

	l := listener{
		addr: *laddr,
		done: make(chan struct{}),
		rmvd: make(chan struct{}),
		rcvr: make(chan net.Conn, 1),
	}

	// Start a goroutine that removes the listener from
	// the cache once the listener is closed.
	go func() {
		<-l.done
		listeners.Lock()
		defer listeners.Unlock()
		delete(listeners.cache, laddr.Name)
		close(l.rmvd)
	}()

	listeners.cache[laddr.Name] = l
	return l, nil
}

// Dial dials a named connection.
//
// Known networks are "memu" (memconn unbuffered).
//
// When the provided network is unknown the operation defers to
// net.Dial.
func (p *Provider) Dial(network, address string) (net.Conn, error) {
	return p.DialContext(nil, network, address)
}

// DialMem dials a named connection.
//
// Known networks are "memu" (memconn unbuffered).
//
// If laddr is nil then a new address is generated using
// time.Now().UnixNano(). Please note that client addresses are
// not required to be unique.
//
// If raddr is nil then the "localhost" endpoint is used on the
// specified network.
func (p *Provider) DialMem(
	network string, laddr, raddr *Addr) (net.Conn, error) {

	return p.DialMemContext(nil, network, laddr, raddr)
}

// DialContext dials a named connection using a
// Go context to provide timeout behavior.
//
// Please see Dial for more information.
func (p *Provider) DialContext(
	ctx context.Context,
	network, address string) (net.Conn, error) {

	switch p.mapNetwork(network) {
	case networkMemu:
		return p.DialMemContext(ctx, network, nil, &Addr{Name: address})
	default:
		if ctx == nil {
			return net.Dial(network, address)
		}
		return (&net.Dialer{}).DialContext(ctx, network, address)
	}
}

// DialMemContext dials a named connection using a
// Go context to provide timeout behavior.
//
// Please see DialMem for more information.
func (p *Provider) DialMemContext(
	ctx context.Context,
	network string,
	laddr, raddr *Addr) (net.Conn, error) {

	var listeners *listenerCache

	switch p.mapNetwork(network) {
	case networkMemu:
		// If laddr is not specified then create one with the current
		// epoch in nanoseconds. This value need not be unique.
		if laddr == nil {
			laddr = &Addr{Name: fmt.Sprintf("%d", time.Now().UnixNano())}
		}

		// If raddr is not specified then set it to the reserved name
		// "localhost".
		if raddr == nil {
			raddr = &Addr{Name: addrLocalhost}
		}

		if laddr.Buffered {
			return nil, &net.OpError{
				Addr:   raddr,
				Source: laddr,
				Net:    network,
				Op:     "dial",
				Err:    errors.New("incompatible network & local address"),
			}
		}
		if raddr.Buffered {
			return nil, &net.OpError{
				Addr:   raddr,
				Source: laddr,
				Net:    network,
				Op:     "dial",
				Err:    errors.New("incompatible network & remote address"),
			}
		}

		listeners = &p.memu
	default:
		return nil, &net.OpError{
			Addr:   raddr,
			Source: laddr,
			Net:    network,
			Op:     "dial",
			Err:    errors.New("unknown network"),
		}
	}

	listeners.RLock()
	defer listeners.RUnlock()

	if l, ok := listeners.cache[raddr.Name]; ok {
		return l.dial(ctx, network, *laddr, *raddr)
	}

	return nil, &net.OpError{
		Addr:   raddr,
		Source: laddr,
		Net:    network,
		Op:     "dial",
		Err:    errors.New("unknown remote address"),
	}
}

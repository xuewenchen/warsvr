package pkg

import (
	"fmt"
	"hash/fnv"
	"math/rand"
	"sync"
	"time"

	"cardwar/pkg/conf"
	"cardwar/protocol"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/zlog"
	"github.com/aceld/zinx/znet"
)

// BackendPool routes a key to one backend connection.
type BackendPool interface {
	Route(key string) ziface.IConnection
}

// BackendRouterConfig pairs a message ID with a router to register on backend connections.
type BackendRouterConfig struct {
	MsgID  uint32
	Router ziface.IRouter
}

// RouteFunc is a pluggable routing strategy. Receives the routing key and healthy connections.
type RouteFunc func(key string, healthy []ziface.IConnection) ziface.IConnection

// DirectRoute finds a connection whose "server_id" property matches the key.
// Used for stateful services where requests must route to a specific instance.
func DirectRoute(key string, healthy []ziface.IConnection) ziface.IConnection {
	for _, conn := range healthy {
		if id, err := conn.GetProperty("server_id"); err == nil {
			if fmt.Sprint(id) == key {
				return conn
			}
		}
	}
	return nil
}

// RouteFuncFor returns the RouteFunc for the given type string.
// Supported types: "hash" (default), "random", "direct".
func RouteFuncFor(routeType string) RouteFunc {
	switch routeType {
	case "random":
		return RandomRoute
	case "direct":
		return DirectRoute
	default:
		return HashRoute
	}
}

// HashRoute picks a connection by hashing the key.
func HashRoute(key string, healthy []ziface.IConnection) ziface.IConnection {
	if len(healthy) == 0 {
		return nil
	}
	h := fnv.New32a()
	h.Write([]byte(key))
	return healthy[int(h.Sum32())%len(healthy)]
}

// RandomRoute picks a connection at random.
func RandomRoute(key string, healthy []ziface.IConnection) ziface.IConnection {
	if len(healthy) == 0 {
		return nil
	}
	return healthy[rand.Intn(len(healthy))]
}

// connEntry holds a single backend connection with reconnect state.
type connEntry struct {
	addr     string
	routers  []BackendRouterConfig
	identity string // caller's service identity (e.g. "gateway"), sent on connect

	mu           sync.Mutex
	conn         ziface.IConnection
	healthy      bool
	stopped      bool // true = removed from pool, reconnect stops
	backoff      time.Duration
	reconnecting bool
	failLogged   bool // only log first reconnect failure
}

// Pool manages a set of backend connections with automatic reconnection.
// The routing strategy is pluggable via RouteFunc.
type Pool struct {
	mu      sync.RWMutex
	conns   []*connEntry
	routeFn RouteFunc
}

// NewPool creates a Pool with initial connections, server info for reconnection, and routing strategy.
func NewPool(conns []ziface.IConnection, servers []conf.ServerNode, routers []BackendRouterConfig, routeFn RouteFunc) *Pool {
	entries := make([]*connEntry, len(conns))
	for i, conn := range conns {
		entries[i] = &connEntry{
			conn:    conn,
			addr:    servers[i].Listen,
			routers: routers,
			healthy: conn != nil,
			backoff: reconnectInitBackoff,
		}
	}
	return &Pool{conns: entries, routeFn: routeFn}
}

// Dial connects to all configured instances of a backend service and returns a Pool.
// The service name matches the key in config.yml services section.
// Routers are registered on every connection. routeFn determines how Route() picks a connection.
func Dial(service string, routers []BackendRouterConfig, routeFn RouteFunc, identity string) *Pool {
	servers := conf.GlobalConfig.Services[service]
	if len(servers) == 0 {
		panic("no " + service + " configured")
	}

	var wg sync.WaitGroup
	pool := &Pool{
		conns:   make([]*connEntry, len(servers)),
		routeFn: routeFn,
	}

	for i, svr := range servers {
		idx := i
		srv := svr

		host, port := conf.ParseHostPort(srv.Listen)
		client := znet.NewClient(host, port)

		client.SetOnConnStart(func(conn ziface.IConnection) {
			if identity != "" {
				conn.SendMsg(protocol.MsgIdServiceIdentity, []byte(identity))
			}
			pool.conns[idx].mu.Lock()
			pool.conns[idx].conn = conn
			pool.conns[idx].healthy = true
			pool.conns[idx].mu.Unlock()
			conn.SetProperty("server_id", srv.ID)
			zlog.Ins().InfoF("Dial: connected to %s[%s]: %s", service, srv.ID, conn.RemoteAddr())
			wg.Done()
		})
		client.SetOnConnStop(func(conn ziface.IConnection) {
			zlog.Ins().InfoF("Dial: disconnected from %s[%s]", service, srv.ID)
			pool.OnDisconnect(idx)
		})

		for _, rc := range routers {
			client.AddRouter(rc.MsgID, rc.Router)
		}

		pool.conns[idx] = &connEntry{
			addr:     srv.Listen,
			routers:  routers,
			identity: identity,
			healthy:  false,
			backoff:  time.Second,
		}

		wg.Add(1)
		client.Start()
	}

	// Wait up to dialTimeout for connections, then proceed (reconnect handles the rest)
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(dialTimeout):
		healthy := len(pool.HealthyConns())
		zlog.Ins().InfoF("Dial: %s proceeding with %d/%d connections", service, healthy, len(servers))
		// Start reconnect loops for entries that never connected
		for i, e := range pool.conns {
			if !e.healthy && !e.stopped && !e.reconnecting {
				pool.OnDisconnect(i)
			}
		}
	}
	return pool
}

// Route selects a backend connection using the configured routing strategy.
func (p *Pool) Route(key string) ziface.IConnection {
	return p.routeFn(key, p.HealthyConns())
}

// AddServer connects to a single new server and appends it to the pool.
// Blocks until the first connection succeeds or times out.
func (p *Pool) AddServer(srv conf.ServerNode, service string, routers []BackendRouterConfig, routeFn RouteFunc, identity string) {
	p.mu.Lock()
	idx := len(p.conns)
	entry := &connEntry{
		addr:     srv.Listen,
		routers:  routers,
		identity: identity,
		backoff:  reconnectInitBackoff,
	}
	p.conns = append(p.conns, entry)
	p.routeFn = routeFn
	p.mu.Unlock()

	host, port := conf.ParseHostPort(srv.Listen)
	client := znet.NewClient(host, port)

	var wg sync.WaitGroup
	wg.Add(1)
	client.SetOnConnStart(func(conn ziface.IConnection) {
		if identity != "" {
			conn.SendMsg(protocol.MsgIdServiceIdentity, []byte(identity))
		}
		entry.mu.Lock()
		if !entry.stopped {
			entry.conn = conn
			entry.healthy = true
		}
		entry.mu.Unlock()
		conn.SetProperty("server_id", srv.ID)
		zlog.Ins().InfoF("Pool: added server %s[%s]: %s", service, srv.ID, conn.RemoteAddr())
		wg.Done()
	})
	client.SetOnConnStop(func(conn ziface.IConnection) {
		zlog.Ins().InfoF("Pool: server disconnected %s[%s]", service, srv.ID)
		p.OnDisconnect(idx)
	})

	for _, rc := range routers {
		client.AddRouter(rc.MsgID, rc.Router)
	}
	client.Start()

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(reconnectTimeout):
		zlog.Ins().ErrorF("Pool: timeout connecting to %s[%s] at %s", service, srv.ID, srv.Listen)
		client.Stop()
		entry.mu.Lock()
		entry.stopped = true
		entry.mu.Unlock()
	}
}

// RemoveServer stops a server by address and marks it for removal from the pool.
func (p *Pool) RemoveServer(addr string) {
	p.mu.RLock()
	var target *connEntry
	for _, e := range p.conns {
		if e.addr == addr && !e.stopped {
			target = e
			break
		}
	}
	p.mu.RUnlock()
	if target == nil {
		return
	}
	target.mu.Lock()
	target.stopped = true
	if target.conn != nil {
		target.conn.Stop()
	}
	target.mu.Unlock()
}

// ServerAddrs returns the listen addresses of all non-stopped pool entries.
func (p *Pool) ServerAddrs() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	var addrs []string
	for _, e := range p.conns {
		if !e.stopped {
			addrs = append(addrs, e.addr)
		}
	}
	return addrs
}

// Sync adds new servers and removes old ones to match the given server list.
func (p *Pool) Sync(servers []conf.ServerNode, service string, routers []BackendRouterConfig, routeFn RouteFunc, identity string) {
	current := make(map[string]bool)
	for _, addr := range p.ServerAddrs() {
		current[addr] = true
	}
	wanted := make(map[string]conf.ServerNode)
	for _, srv := range servers {
		wanted[srv.Listen] = srv
	}

	// Add new servers
	for addr, srv := range wanted {
		if !current[addr] {
			zlog.Ins().InfoF("Pool: syncing new server %s[%s] at %s", service, srv.ID, addr)
			p.AddServer(srv, service, routers, routeFn, identity)
		}
	}

	// Remove servers no longer configured
	for addr := range current {
		if _, ok := wanted[addr]; !ok {
			zlog.Ins().InfoF("Pool: removing server %s %s", service, addr)
			p.RemoveServer(addr)
		}
	}
}

// HealthyConns returns all currently healthy connections. Safe for concurrent use.
func (p *Pool) HealthyConns() []ziface.IConnection {
	p.mu.RLock()
	defer p.mu.RUnlock()
	healthy := make([]ziface.IConnection, 0, len(p.conns))
	for _, e := range p.conns {
		if e.healthy && e.conn != nil && !e.stopped {
			healthy = append(healthy, e.conn)
		}
	}
	return healthy
}

// OnDisconnect marks a connection as dead and starts reconnection.
func (p *Pool) OnDisconnect(idx int) {
	p.mu.RLock()
	e := p.conns[idx]
	p.mu.RUnlock()

	e.mu.Lock()
	e.healthy = false
	e.conn = nil
	if e.reconnecting || e.stopped {
		e.mu.Unlock()
		return
	}
	e.reconnecting = true
	e.mu.Unlock()

	go p.reconnectLoop(idx)
}

const (
	dialTimeout          = 3 * time.Second
	reconnectTimeout     = 5 * time.Second
	reconnectMaxBackoff  = 5 * time.Second
	reconnectInitBackoff = 200 * time.Millisecond
)

func (p *Pool) reconnectLoop(idx int) {
	p.mu.RLock()
	e := p.conns[idx]
	p.mu.RUnlock()

	for {
		e.mu.Lock()
		delay := e.backoff
		e.mu.Unlock()

		time.Sleep(delay)

		host, port := conf.ParseHostPort(e.addr)
		client := znet.NewClient(host, port)

		connCh := make(chan ziface.IConnection, 1)

		client.SetOnConnStart(func(conn ziface.IConnection) {
			if e.identity != "" {
				conn.SendMsg(protocol.MsgIdServiceIdentity, []byte(e.identity))
			}
			zlog.Ins().InfoF("Pool: reconnected to %s", e.addr)
			connCh <- conn
		})
		client.SetOnConnStop(func(conn ziface.IConnection) {
			// OnDisconnect handles future drops after reconnect success.
			p.OnDisconnect(idx)
			// Signal the reconnect loop to retry (non-blocking).
			select {
			case connCh <- nil:
			default:
			}
		})

		for _, rc := range e.routers {
			client.AddRouter(rc.MsgID, rc.Router)
		}

		client.Start()

		select {
		case conn := <-connCh:
			if conn != nil {
				e.mu.Lock()
				e.conn = conn
				e.healthy = true
				e.backoff = reconnectInitBackoff
				e.reconnecting = false
				e.failLogged = false
				e.mu.Unlock()
				return
			}
		case <-time.After(reconnectTimeout):
			client.Stop()
		}

		e.mu.Lock()
		if e.stopped {
			e.mu.Unlock()
			return
		}
		if !e.failLogged {
			zlog.Ins().ErrorF("Pool: reconnect failed to %s, retrying (backoff: %v)", e.addr, e.backoff)
			e.failLogged = true
		}
		if e.backoff < reconnectMaxBackoff {
			e.backoff *= 2
		}
		e.mu.Unlock()
	}
}

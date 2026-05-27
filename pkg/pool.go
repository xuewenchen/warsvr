package pkg

import (
	"hash/fnv"
	"math/rand"
	"sync"
	"time"

	"cardwar/conf"

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
	addr          string
	routers       []BackendRouterConfig
	registerMsgID uint32

	mu           sync.Mutex
	conn         ziface.IConnection
	healthy      bool
	backoff      time.Duration
	reconnecting bool
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
			backoff: time.Second,
		}
	}
	return &Pool{conns: entries, routeFn: routeFn}
}

// Dial connects to all configured instances of a backend service and returns a Pool.
// The service name matches the key in config.yml services section.
// Routers are registered on every connection. routeFn determines how Route() picks a connection.
func Dial(service string, routers []BackendRouterConfig, routeFn RouteFunc, registerMsgID uint32) *Pool {
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
			if registerMsgID != 0 {
				conn.SendMsg(registerMsgID, []byte{})
			}
			pool.conns[idx].mu.Lock()
			pool.conns[idx].conn = conn
			pool.conns[idx].healthy = true
			pool.conns[idx].mu.Unlock()
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
			addr:          srv.Listen,
			routers:       routers,
			registerMsgID: registerMsgID,
			healthy:       false,
			backoff:       time.Second,
		}

		wg.Add(1)
		client.Start()
	}

	wg.Wait()
	return pool
}

// Route selects a backend connection using the configured routing strategy.
func (p *Pool) Route(key string) ziface.IConnection {
	return p.routeFn(key, p.HealthyConns())
}

// HealthyConns returns all currently healthy connections. Safe for concurrent use.
func (p *Pool) HealthyConns() []ziface.IConnection {
	p.mu.RLock()
	defer p.mu.RUnlock()
	healthy := make([]ziface.IConnection, 0, len(p.conns))
	for _, e := range p.conns {
		if e.healthy && e.conn != nil {
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
	if e.reconnecting {
		e.mu.Unlock()
		return
	}
	e.reconnecting = true
	e.mu.Unlock()

	go p.reconnectLoop(idx)
}

const reconnectTimeout = 15 * time.Second

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
			if e.registerMsgID != 0 {
				conn.SendMsg(e.registerMsgID, []byte{})
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
				e.backoff = time.Second
				e.reconnecting = false
				e.mu.Unlock()
				return
			}
		case <-time.After(reconnectTimeout):
			client.Stop()
		}

		e.mu.Lock()
		if e.backoff < 30*time.Second {
			e.backoff *= 2
		}
		e.mu.Unlock()
	}
}

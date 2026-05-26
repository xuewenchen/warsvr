package common

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
	addr    string
	routers []BackendRouterConfig

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

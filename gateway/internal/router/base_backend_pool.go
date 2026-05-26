package router

import (
	"sync"
	"time"

	"cardwar/conf"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/zlog"
	"github.com/aceld/zinx/znet"
)

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

// BaseBackendPool manages a set of backend connections with automatic reconnection.
// Embed this in service-specific pools that implement the BackendPool interface.
type BaseBackendPool struct {
	mu    sync.RWMutex
	conns []*connEntry
}

func newBaseBackendPool(conns []ziface.IConnection, servers []conf.ServerNode, routers []BackendRouterConfig) *BaseBackendPool {
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
	bp := &BaseBackendPool{conns: entries}
	return bp
}

// HealthyConns returns all currently healthy connections. Safe for concurrent use.
func (p *BaseBackendPool) HealthyConns() []ziface.IConnection {
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

func (p *BaseBackendPool) getBase() *BaseBackendPool { return p }

// onDisconnect marks a connection as dead and starts reconnection.
func (p *BaseBackendPool) onDisconnect(idx int) {
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

func (p *BaseBackendPool) reconnectLoop(idx int) {
	p.mu.RLock()
	e := p.conns[idx]
	p.mu.RUnlock()

	for {
		e.mu.Lock()
		backoff := e.backoff
		e.mu.Unlock()

		time.Sleep(backoff)

		host, port := conf.ParseHostPort(e.addr)
		client := znet.NewClient(host, port)

		reconnectCh := make(chan ziface.IConnection, 1)

		client.SetOnConnStart(func(conn ziface.IConnection) {
			zlog.Ins().InfoF("BackendPool: reconnected to %s", e.addr)
			reconnectCh <- conn
		})
		client.SetOnConnStop(func(conn ziface.IConnection) {
			p.onDisconnect(idx)
		})

		for _, rc := range e.routers {
			client.AddRouter(rc.MsgID, rc.Router)
		}

		client.Start()

		select {
		case conn := <-reconnectCh:
			e.mu.Lock()
			e.conn = conn
			e.healthy = true
			e.backoff = time.Second
			e.reconnecting = false
			e.mu.Unlock()
			return
		case <-time.After(30 * time.Second):
			e.mu.Lock()
			if e.backoff < 30*time.Second {
				e.backoff *= 2
			}
			e.mu.Unlock()
		}
	}
}

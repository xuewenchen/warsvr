package pkg

import (
	"cardwar/pkg/conf"
	"sync"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/zlog"
)

// Registry manages connections to multiple backend services.
// Any service can use it to Dial backends and RouteTo them.
type Registry struct {
	mu       sync.RWMutex
	backends map[string]BackendPool
	identity string // e.g. "gateway", "roomsvr" — auto-sent on connect
}

// NewRegistry creates a new Registry. identity is the caller's service type
// (e.g. "gateway", "roomsvr"), automatically exchanged on each Dial connection.
func NewRegistry(identity string) *Registry {
	return &Registry{backends: make(map[string]BackendPool), identity: identity}
}

// Dial connects to all configured instances of a backend service and stores the pool.
// The Registry's identity is automatically sent on each connection so the backend
// can identify the caller.
func (r *Registry) Dial(service string, routers []BackendRouterConfig, routeFn RouteFunc) {
	pool := Dial(service, routers, routeFn, r.identity)
	r.mu.Lock()
	r.backends[service] = pool
	r.mu.Unlock()
}

// RouteTo routes a key to a connection in the named backend pool.
func (r *Registry) RouteTo(backend, key string) ziface.IConnection {
	r.mu.RLock()
	p := r.backends[backend]
	r.mu.RUnlock()
	if p == nil {
		return nil
	}
	return p.Route(key)
}

// Pool returns the BackendPool for the named service, or nil.
func (r *Registry) Pool(backend string) BackendPool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.backends[backend]
}

// CleanupBackends closes and removes backend pools whose service name is not
// present in the keep map. Call after SyncBackend to garbage-collect backends
// that were entirely removed from config.
func (r *Registry) CleanupBackends(keep map[string]struct{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for name, pool := range r.backends {
		if _, ok := keep[name]; !ok {
			zlog.Ins().InfoF("Registry: cleaning up removed backend %s", name)
			pool.Close()
			delete(r.backends, name)
		}
	}
}

// SyncBackend adds new servers and removes old ones for a backend service.
func (r *Registry) SyncBackend(service string, routers []BackendRouterConfig, routeFn RouteFunc) {
	servers := conf.GlobalConfig.Services[service]
	r.mu.RLock()
	pool := r.backends[service]
	r.mu.RUnlock()
	if pool == nil {
		return
	}
	if p, ok := pool.(*Pool); ok {
		p.Sync(servers, service, routers, routeFn, r.identity)
	}
}

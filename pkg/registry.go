package pkg

import (
	"sync"

	"github.com/aceld/zinx/ziface"
)

// Registry manages connections to multiple backend services.
// Any service can use it to Dial backends and RouteTo them.
type Registry struct {
	mu       sync.RWMutex
	backends map[string]BackendPool
}

// NewRegistry creates a new Registry.
func NewRegistry() *Registry {
	return &Registry{backends: make(map[string]BackendPool)}
}

// Dial connects to all configured instances of a backend service and stores the pool.
func (r *Registry) Dial(service string, routers []BackendRouterConfig, routeFn RouteFunc) {
	pool := Dial(service, routers, routeFn)
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

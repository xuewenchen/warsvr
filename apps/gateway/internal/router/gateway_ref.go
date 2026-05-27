package router

import (
	"cardwar/conf"
	"cardwar/pkg"
	"sync"

	"github.com/aceld/zinx/ziface"
)

// BackendRouteInfo holds routing information for a single message ID.
type BackendRouteInfo struct {
	Backend  string // backend service name (e.g. "chatsvr")
	RouteKey string // "connId" or "playerId"
}

// GatewayRef holds Gateway-specific state. Embeds Registry for backend connection management.
type GatewayRef struct {
	*pkg.Registry
	Server      ziface.IServer
	PlayerConns *sync.Map // playerID → connID (uint64)

	mu     sync.RWMutex
	routes map[uint32]*BackendRouteInfo
}

// RouteFor returns the backend route for a given message ID, or nil if not found.
func (gw *GatewayRef) RouteFor(msgID uint32) *BackendRouteInfo {
	gw.mu.RLock()
	defer gw.mu.RUnlock()
	return gw.routes[msgID]
}

// SetRoutes atomically replaces the route table (for hot-reload).
func (gw *GatewayRef) SetRoutes(r map[uint32]*BackendRouteInfo) {
	gw.mu.Lock()
	gw.routes = r
	gw.mu.Unlock()
}

// BuildRouteIndex builds the forward route lookup table from config.
func BuildRouteIndex(cfg conf.GatewayConfig) map[uint32]*BackendRouteInfo {
	routes := make(map[uint32]*BackendRouteInfo)
	for backend, rc := range cfg.Routes {
		for _, msgID := range rc.Forward {
			routes[msgID] = &BackendRouteInfo{Backend: backend, RouteKey: rc.RouteKey}
		}
	}
	return routes
}

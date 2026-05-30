package router

import (
	"cardwar/pkg"
	"cardwar/pkg/conf"
	"cardwar/protocol"
	"sync"

	"github.com/aceld/zinx/ziface"
)

// BackendRouteInfo holds routing information for a single message ID.
type BackendRouteInfo struct {
	Backend   string // backend service name (e.g. "chatsvr")
	RouteKey  string // "connId", "playerId", or custom property name
	RouteType string // "hash" (default) or "random" — passed to pkg.Dial
}

// GatewayRef holds Gateway-specific state. Embeds Registry for backend connection management.
type GatewayRef struct {
	*pkg.Registry
	ID          string // instance ID (e.g. "gw-1")
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

// DialSessionSvr connects to all configured SessionSvr instances and registers
// session response routers (SessionGet, SessionReconnect) on each connection.
func (gw *GatewayRef) DialSessionSvr() {
	gw.Registry.Dial(conf.SvcSessionSvr, gw.sessionRouters(), pkg.HashRoute)
}

// SyncSessionSvr syncs the SessionSvr backend pool during hot-reload.
func (gw *GatewayRef) SyncSessionSvr() {
	gw.Registry.SyncBackend(conf.SvcSessionSvr, gw.sessionRouters(), pkg.HashRoute)
}

// sessionRouters builds the BackendRouterConfig slice for SessionSvr connections.
func (gw *GatewayRef) sessionRouters() []pkg.BackendRouterConfig {
	return []pkg.BackendRouterConfig{
		{MsgID: protocol.MsgIdSessionGet, Router: &SessionResponseRouter{GW: gw}},
		{MsgID: protocol.MsgIdSessionReconnect, Router: &SessionResponseRouter{GW: gw}},
	}
}

// BuildRouteIndex builds the forward route lookup table from config.
func BuildRouteIndex(cfg conf.GatewayConfig) map[uint32]*BackendRouteInfo {
	routes := make(map[uint32]*BackendRouteInfo)
	for backend, rc := range cfg.Routes {
		for _, msgID := range rc.Forward {
			routes[msgID] = &BackendRouteInfo{Backend: backend, RouteKey: rc.RouteKey, RouteType: rc.RouteType}
		}
	}
	return routes
}

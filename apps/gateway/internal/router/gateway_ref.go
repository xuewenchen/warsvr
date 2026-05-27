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
	Server        ziface.IServer
	PlayerConns   *sync.Map                 // playerID → connID (uint64)
	ForwardRoutes map[uint32]*BackendRouteInfo
}

// RouteFor returns the backend route for a given message ID, or nil if not found.
func (gw *GatewayRef) RouteFor(msgID uint32) *BackendRouteInfo {
	return gw.ForwardRoutes[msgID]
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

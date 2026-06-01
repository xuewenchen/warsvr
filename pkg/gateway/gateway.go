package gateway

import (
	"cardwar/pkg"
	"cardwar/pkg/conf"
	"cardwar/pkg/corouter"
	"cardwar/protocol"
	"sync"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/zlog"
)

// BackendRouteInfo holds routing information for a single message ID.
type BackendRouteInfo struct {
	Backend   string // backend service name (e.g. "chatsvr")
	RouteKey  string // "connId", "playerId", or custom property name
	RouteType string // "hash" (default) or "random" — passed to pkg.Dial
}

// BackendRouteCfg stores per-backend routing strategy from config.yml (without forward list).
type BackendRouteCfg struct {
	RouteKey  string
	RouteType string
}

// GatewayServer holds Gateway-specific state. Embeds Registry for backend connection management.
type GatewayServer struct {
	*pkg.Registry
	ID          string // instance ID (e.g. "gw-1")
	Server      ziface.IServer
	PlayerConns *sync.Map // playerID → connID (uint64)

	mu            sync.RWMutex
	routes        map[uint32]*BackendRouteInfo
	backendCfgs   map[string]BackendRouteCfg // backend → routing strategy (from config)
	fwdRegistered map[uint32]bool            // msgIDs already registered on Server

	FwdRouter *ForwardRouter  // set during init
	RspRouter *ResponseRouter // set during init
}

// New creates a GatewayServer with all initialization: config loading, backend connections,
// config watch, and WebSocket server setup. Returns a ready-to-serve gateway.
func New(configPath, gwID string) (*GatewayServer, error) {
	if err := conf.Load(configPath); err != nil {
		return nil, err
	}

	gw := &GatewayServer{
		Registry:      pkg.NewRegistry(conf.SvcGateway),
		ID:            gwID,
		PlayerConns:   &sync.Map{},
		fwdRegistered: make(map[uint32]bool),
	}

	routeIndex, backendCfgs := BuildRouteIndex(conf.GlobalConfig.Gateway)
	gw.SetRoutes(routeIndex)
	gw.SetBackendCfgs(backendCfgs)

	helloRouter := &ServiceHelloRouter{GW: gw}
	gw.InitResponse(&ResponseRouter{GW: gw})

	// 初始化websocket（必须在 Dial 之前，确保 Server 和 FwdRouter 就绪）
	initWebSocket(gw, gwID)

	// 链接其他后端服务
	for backend, rc := range conf.GlobalConfig.Gateway.Routes {
		routers := gw.backendRouters(helloRouter)
		gw.Dial(backend, routers, pkg.RouteFuncFor(rc.RouteType))
	}
	// 链接session后端服务
	gw.DialSessionSvr()

	// 配置热更
	if _, err := conf.Watch(configPath, func(cfg *conf.Config) {
		// 同步其他后端服务
		newIndex, backendCfgs := BuildRouteIndex(cfg.Gateway)
		gw.SetRoutes(newIndex)
		gw.SetBackendCfgs(backendCfgs)
		for backend, rc := range cfg.Gateway.Routes {
			routers := gw.backendRouters(helloRouter)
			gw.SyncBackend(backend, routers, pkg.RouteFuncFor(rc.RouteType))
		}
		// 同步session服务
		gw.SyncSessionSvr()
		zlog.Ins().InfoF("Gateway: hot-reloaded (%d msgIDs, %d backends)", len(newIndex), len(cfg.Gateway.Routes))
	}); err != nil {
		zlog.Ins().ErrorF("Gateway: config watch failed: %v", err)
	}

	return gw, nil
}

// RouteFor returns the backend route for a given message ID, or nil if not found.
func (gw *GatewayServer) RouteFor(msgID uint32) *BackendRouteInfo {
	gw.mu.RLock()
	defer gw.mu.RUnlock()
	return gw.routes[msgID]
}

// SetRoutes atomically replaces the route table.
func (gw *GatewayServer) SetRoutes(r map[uint32]*BackendRouteInfo) {
	gw.mu.Lock()
	gw.routes = r
	gw.mu.Unlock()
}

// SetBackendCfgs atomically replaces the backend config map.
func (gw *GatewayServer) SetBackendCfgs(cfgs map[string]BackendRouteCfg) {
	gw.mu.Lock()
	gw.backendCfgs = cfgs
	gw.mu.Unlock()
}

// InitForward sets the ForwardRouter.
func (gw *GatewayServer) InitForward(fwd *ForwardRouter) {
	gw.FwdRouter = fwd
}

// InitResponse sets the ResponseRouter for dynamic registration on backend connections.
func (gw *GatewayServer) InitResponse(rsp *ResponseRouter) {
	gw.RspRouter = rsp
}

// backendRouters returns the common router list for backend connections.
func (gw *GatewayServer) backendRouters(helloRouter *ServiceHelloRouter) []pkg.BackendRouterConfig {
	return []pkg.BackendRouterConfig{
		{MsgID: protocol.MsgIdPing, Router: &corouter.PingRouter{}},
		{MsgID: protocol.MsgIdServiceHello, Router: helloRouter},
	}
}

// DialSessionSvr connects to all configured SessionSvr instances.
func (gw *GatewayServer) DialSessionSvr() {
	gw.Registry.Dial(conf.SvcSessionSvr, gw.sessionRouters(), pkg.HashRoute)
}

// SyncSessionSvr syncs the SessionSvr backend pool during hot-reload.
func (gw *GatewayServer) SyncSessionSvr() {
	gw.Registry.SyncBackend(conf.SvcSessionSvr, gw.sessionRouters(), pkg.HashRoute)
}

// sessionRouters builds the BackendRouterConfig slice for SessionSvr connections.
func (gw *GatewayServer) sessionRouters() []pkg.BackendRouterConfig {
	return []pkg.BackendRouterConfig{
		{MsgID: protocol.MsgIdPing, Router: &corouter.PingRouter{}},
		{MsgID: protocol.MsgIdSessionGet, Router: &SessionResponseRouter{GW: gw}},
		{MsgID: protocol.MsgIdSessionReconnect, Router: &SessionResponseRouter{GW: gw}},
	}
}

// BuildRouteIndex builds the forward route lookup table and backend config map from config.
func BuildRouteIndex(cfg conf.GatewayConfig) (routes map[uint32]*BackendRouteInfo, backendCfgs map[string]BackendRouteCfg) {
	routes = make(map[uint32]*BackendRouteInfo)
	backendCfgs = make(map[string]BackendRouteCfg)
	for backend, rc := range cfg.Routes {
		backendCfgs[backend] = BackendRouteCfg{RouteKey: rc.RouteKey, RouteType: rc.RouteType}
		for _, msgID := range rc.Forward {
			routes[msgID] = &BackendRouteInfo{Backend: backend, RouteKey: rc.RouteKey, RouteType: rc.RouteType}
		}
	}
	return
}

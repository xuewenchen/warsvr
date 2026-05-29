package main

import (
	"cardwar/apps/gateway/internal/router"
	"cardwar/pkg"
	"cardwar/pkg/auth"
	"cardwar/pkg/conf"
	"cardwar/pkg/corouter"
	"cardwar/protocol"
	"flag"
	"net/http"
	"sync"

	"github.com/aceld/zinx/zconf"
	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/zlog"
	"github.com/aceld/zinx/znet"
)

var sessionRouters = []pkg.BackendRouterConfig{
	{MsgID: protocol.MsgIdSessionGet, Router: &router.SessionResponseRouter{}},
	{MsgID: protocol.MsgIdSessionReconnect, Router: &router.SessionResponseRouter{}},
}

func main() {
	configPath := flag.String("conf", "config.yml", "path to config file")
	gwID := flag.String("id", "", "Gateway ID (matches config services.gateway[].id)")
	flag.Parse()

	if err := conf.Load(*configPath); err != nil {
		panic(err)
	}

	routeIndex := router.BuildRouteIndex(conf.GlobalConfig.Gateway)

	gw := &router.GatewayRef{
		Registry:    pkg.NewRegistry(conf.SvcGateway),
		PlayerConns: &sync.Map{},
	}
	gw.SetRoutes(routeIndex)

	// Wire GW reference into session response routers
	for _, rc := range sessionRouters {
		rc.Router.(*router.SessionResponseRouter).GW = gw
	}

	rspRouter := &router.ResponseRouter{GW: gw}
	registerResponseRouter(gw, rspRouter)

	if _, err := conf.Watch(*configPath, func(cfg *conf.Config) {
		newIndex := router.BuildRouteIndex(cfg.Gateway)
		gw.SetRoutes(newIndex)
		for backend, rc := range cfg.Gateway.Routes {
			routers := make([]pkg.BackendRouterConfig, 0, pkg.MaxMsgID)
			for msgID := uint32(1); msgID <= pkg.MaxMsgID; msgID++ {
				routers = append(routers, pkg.BackendRouterConfig{MsgID: msgID, Router: rspRouter})
			}
			gw.SyncBackend(backend, routers, pkg.RouteFuncFor(rc.RouteType))
		}
		gw.SyncBackend(conf.SvcSessionSvr, sessionRouters, pkg.HashRoute)
		zlog.Ins().InfoF("Gateway: hot-reloaded (%d msgIDs, %d backends)", len(newIndex), len(cfg.Gateway.Routes))
	}); err != nil {
		zlog.Ins().ErrorF("Gateway: config watch failed: %v", err)
	}

	initWebSocket(gw, *gwID)
	gw.Server.Serve()
}

// 初始化websocket服务器，设置认证、连接管理和消息路由
func initWebSocket(gw *router.GatewayRef, gwID string) {
	gwCfg := conf.LookupServer(conf.GlobalConfig.Services[conf.SvcGateway], gwID, conf.SvcGateway)
	jwtSecret := conf.GlobalConfig.Gateway.JWTSecret

	_, wsPort := conf.ParseHostPort(gwCfg.WSListen)
	tcpHost, tcpPort := conf.ParseHostPort(gwCfg.TCPListen)

	serverCfg := &zconf.Config{
		Name:    conf.SvcGateway,
		Host:    tcpHost,
		TCPPort: tcpPort,
		WsPort:  wsPort,
		WsPath:  "/ws",
		Mode:    "tcp,ws",
	}
	wsServer := znet.NewUserConfServer(serverCfg)
	gw.Server = wsServer

	var pendingAuths sync.Map

	// 设置鉴权
	wsServer.SetWebsocketAuth(func(r *http.Request) error {
		token := r.URL.Query().Get("token")
		if token == "" {
			return pkg.ErrUnauthorized("missing token")
		}
		playerID, err := auth.ValidateJWT(token, jwtSecret)
		if err != nil {
			zlog.Ins().ErrorF("Gateway: JWT validation failed for %s: %v", r.RemoteAddr, err)
			return pkg.ErrUnauthorized("invalid token")
		}
		pendingAuths.Store(r.RemoteAddr, playerID)
		zlog.Ins().InfoF("Gateway: JWT validated for player %d from %s", playerID, r.RemoteAddr)
		return nil
	})

	// 设置客户端链接启动
	wsServer.SetOnConnStart(func(conn ziface.IConnection) {
		addr := conn.RemoteAddr().String()
		val, ok := pendingAuths.LoadAndDelete(addr)
		if !ok {
			zlog.Ins().ErrorF("Gateway: unauthenticated connection from %s, closing", addr)
			conn.Stop()
			return
		}
		playerID := val.(int64)
		conn.SetProperty("playerId", playerID)
		gw.PlayerConns.Store(playerID, conn.GetConnID())
		zlog.Ins().InfoF("Client connected: connID=%d, player=%d, addr=%s", conn.GetConnID(), playerID, addr)

		gw.CheckReconnect(playerID, conn)
	})

	// 设置玩家链接断开
	wsServer.SetOnConnStop(func(conn ziface.IConnection) {
		if pidVal, err := conn.GetProperty("playerId"); err == nil {
			if pid, ok := pidVal.(int64); ok {
				gw.PlayerConns.Delete(pid)
				gw.MarkDisconnected(pid)
			}
		}
		zlog.Ins().InfoF("Client disconnected: connID=%d", conn.GetConnID())
	})

	wsServer.AddRouter(protocol.MsgIdPing, &corouter.PingRouter{})

	registerForwardRouter(gw)
}

// 注册响应路由
func registerResponseRouter(gw *router.GatewayRef, rspRouter *router.ResponseRouter) {
	for backend, rc := range conf.GlobalConfig.Gateway.Routes {
		routers := make([]pkg.BackendRouterConfig, 0, pkg.MaxMsgID)
		for msgID := uint32(1); msgID <= pkg.MaxMsgID; msgID++ {
			routers = append(routers, pkg.BackendRouterConfig{MsgID: msgID, Router: rspRouter})
		}
		gw.Dial(backend, routers, pkg.RouteFuncFor(rc.RouteType))
	}
	gw.Dial(conf.SvcSessionSvr, sessionRouters, pkg.HashRoute)
}

// 注册转发路由
func registerForwardRouter(gw *router.GatewayRef) {
	fwdRouter := &router.ForwardRouter{GW: gw}
	for msgID := uint32(1); msgID <= pkg.MaxMsgID; msgID++ {
		if msgID == protocol.MsgIdPing {
			continue
		}
		gw.Server.AddRouter(msgID, fwdRouter)
	}
}

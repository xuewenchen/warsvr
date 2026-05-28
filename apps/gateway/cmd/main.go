package main

import (
	"cardwar/apps/gateway/internal/router"
	"cardwar/pkg"
	"cardwar/pkg/auth"
	"cardwar/pkg/conf"
	"cardwar/protocol"
	"flag"
	"net/http"
	"sync"

	"github.com/aceld/zinx/zconf"
	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/zlog"
	"github.com/aceld/zinx/znet"
)

func main() {
	configPath := flag.String("conf", "config.yml", "path to config file")
	gwID := flag.String("id", "", "Gateway ID (matches config services.gateway[].id)")
	flag.Parse()

	if err := conf.Load(*configPath); err != nil {
		panic(err)
	}

	routeIndex := router.BuildRouteIndex(conf.GlobalConfig.Gateway)

	// 初始化网关服务
	gw := &router.GatewayRef{
		Registry:    pkg.NewRegistry(),
		PlayerConns: &sync.Map{},
	}
	gw.SetRoutes(routeIndex)

	// 注册TCP响应路由
	rspRouter := &router.ResponseRouter{GW: gw}
	registerResponseRouter(gw, rspRouter)

	if _, err := conf.Watch(*configPath, func(cfg *conf.Config) {
		newIndex := router.BuildRouteIndex(cfg.Gateway)
		gw.SetRoutes(newIndex)
		for backend := range cfg.Gateway.Routes {
			routers := make([]pkg.BackendRouterConfig, 0, pkg.MaxMsgID)
			for msgID := uint32(1); msgID <= pkg.MaxMsgID; msgID++ {
				routers = append(routers, pkg.BackendRouterConfig{MsgID: msgID, Router: rspRouter})
			}
			gw.SyncBackend(backend, routers, pkg.HashRoute, protocol.MsgIdGatewayRegister)
		}
		zlog.Ins().InfoF("Gateway: hot-reloaded (%d msgIDs, %d backends)", len(newIndex), len(cfg.Gateway.Routes))
	}); err != nil {
		zlog.Ins().ErrorF("Gateway: config watch failed: %v", err)
	}

	initWebSocket(gw, *gwID)
	gw.Server.Serve()
}

func initWebSocket(gw *router.GatewayRef, gwID string) {
	gwCfg := conf.LookupServer(conf.GlobalConfig.Services["gateway"], gwID, "Gateway")
	jwtSecret := conf.GlobalConfig.Gateway.JWTSecret

	_, wsPort := conf.ParseHostPort(gwCfg.WSListen)
	tcpHost, tcpPort := conf.ParseHostPort(gwCfg.TCPListen)

	serverCfg := &zconf.Config{
		Name:    "Gateway",
		Host:    tcpHost,
		TCPPort: tcpPort,
		WsPort:  wsPort,
		WsPath:  "/ws",
		Mode:    "tcp,ws",
	}
	wsServer := znet.NewUserConfServer(serverCfg)
	gw.Server = wsServer

	var pendingAuths sync.Map // remoteAddr -> playerId (passes JWT result from websocketAuth to OnConnStart)

	// 鉴权
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

	// 建立链接
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
	})

	wsServer.SetOnConnStop(func(conn ziface.IConnection) {
		if pid, err := conn.GetProperty("playerId"); err == nil {
			gw.PlayerConns.Delete(pid)
		}
		zlog.Ins().InfoF("Client disconnected: connID=%d", conn.GetConnID())
	})

	wsServer.AddRouter(protocol.MsgIdPing, &router.PingRouter{})

	// 注册websocket消息转发路由
	registerForwardRouter(gw)
}

// 注册backend TCP响应路由，负责将后端服务的响应消息转发给正确的客户端连接
func registerResponseRouter(gw *router.GatewayRef, rspRouter *router.ResponseRouter) {
	for backend := range conf.GlobalConfig.Gateway.Routes {
		routers := make([]pkg.BackendRouterConfig, 0, pkg.MaxMsgID)
		for msgID := uint32(1); msgID <= pkg.MaxMsgID; msgID++ {
			routers = append(routers, pkg.BackendRouterConfig{MsgID: msgID, Router: rspRouter})
		}
		gw.Dial(backend, routers, pkg.HashRoute, protocol.MsgIdGatewayRegister)
	}
}

// 注册websocket消息应该转发到哪个后端服务
func registerForwardRouter(gw *router.GatewayRef) {
	fwdRouter := &router.ForwardRouter{GW: gw}
	for msgID := uint32(1); msgID <= pkg.MaxMsgID; msgID++ {
		if msgID == protocol.MsgIdPing {
			continue // Ping is handled locally
		}
		gw.Server.AddRouter(msgID, fwdRouter)
	}
}

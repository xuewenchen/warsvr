package main

import (
	"cardwar/apps/gateway/internal/router"
	"cardwar/conf"
	"cardwar/pkg"
	"cardwar/protocol"
	"flag"
	"sync"

	"github.com/aceld/zinx/zconf"
	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/zlog"
	"github.com/aceld/zinx/znet"
)

const maxMsgID = 1000

func main() {
	configPath := flag.String("conf", "config.yml", "path to config file")
	gwID := flag.String("id", "", "Gateway ID (matches config services.gateway[].id)")
	flag.Parse()

	if err := conf.Load(*configPath); err != nil {
		panic(err)
	}

	routeIndex := router.BuildRouteIndex(conf.GlobalConfig.Gateway)

	gw := &router.GatewayRef{
		Registry:    pkg.NewRegistry(),
		PlayerConns: &sync.Map{},
	}
	gw.SetRoutes(routeIndex)

	rspRouter := &router.ResponseRouter{GW: gw}
	for backend, rc := range conf.GlobalConfig.Gateway.Routes {
		routers := make([]pkg.BackendRouterConfig, len(rc.Response))
		for i, msgID := range rc.Response {
			routers[i] = pkg.BackendRouterConfig{MsgID: msgID, Router: rspRouter}
		}
		gw.Dial(backend, routers, pkg.HashRoute)
	}

	if _, err := conf.Watch(*configPath, func(cfg *conf.Config) {
		newIndex := router.BuildRouteIndex(cfg.Gateway)
		gw.SetRoutes(newIndex)
		zlog.Ins().InfoF("Gateway: routes hot-reloaded (%d msgIDs)", len(newIndex))
	}); err != nil {
		zlog.Ins().ErrorF("Gateway: config watch failed: %v", err)
	}

	initWebSocket(gw, *gwID)
	gw.Server.Serve()
}

func initWebSocket(gw *router.GatewayRef, gwID string) {
	gwCfg := conf.LookupServer(conf.GlobalConfig.Services["gateway"], gwID, "Gateway")

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

	wsServer.SetOnConnStart(func(conn ziface.IConnection) {
		zlog.Ins().InfoF("Client connected: connID=%d, addr=%s", conn.GetConnID(), conn.RemoteAddr())
	})
	wsServer.SetOnConnStop(func(conn ziface.IConnection) {
		if pid, err := conn.GetProperty("playerId"); err == nil {
			gw.PlayerConns.Delete(pid)
		}
		zlog.Ins().InfoF("Client disconnected: connID=%d", conn.GetConnID())
	})

	// Ping handled locally
	wsServer.AddRouter(protocol.MsgIdPing, &pingRouter{})

	// Pre-register ForwardRouter for msgIDs 1..maxMsgID so new msgIDs
	// added to config don't require Gateway restart.
	fwdRouter := &router.ForwardRouter{GW: gw}
	for msgID := uint32(1); msgID <= maxMsgID; msgID++ {
		wsServer.AddRouter(msgID, fwdRouter)
	}
}

type pingRouter struct {
	znet.BaseRouter
}

func (r *pingRouter) Handle(request ziface.IRequest) {
	zlog.Ins().DebugF("Call PingRouter Handle")
	request.GetConnection().SendMsg(protocol.MsgIdPong, []byte("pong-server"))
}

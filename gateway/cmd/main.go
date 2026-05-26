package main

import (
	"cardwar/common"
	"cardwar/conf"
	"cardwar/gateway/internal/router"
	"flag"
	"sync"

	"github.com/aceld/zinx/zconf"
	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/zlog"
	"github.com/aceld/zinx/znet"
)

func main() {
	configPath := flag.String("conf", "config.yml", "path to config file")
	flag.Parse()

	if err := conf.Load(*configPath); err != nil {
		panic(err)
	}

	gw := &router.GatewayRef{
		PlayerConns: &sync.Map{},
	}
	// 初始化后台服务
	initBackendSvr(gw)
	// 初始化websocket
	initWebSocket(gw)

	// 启动
	gw.Server.Serve()
}

func initWebSocket(gw *router.GatewayRef) {
	gwCfg := conf.GlobalConfig.Services["gateway"][0]
	// WS+TCP server for clients
	_, wsPort := conf.ParseHostPort(gwCfg.WSListen)
	tcpHost, tcpPort := conf.ParseHostPort(gwCfg.TCPListen)

	serverCfg := &zconf.Config{
		Name:    "Gateway",
		Host:    tcpHost,
		TCPPort: tcpPort,
		WsPort:  wsPort,
		WsPath:  "/ws",
		Mode:    "",
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

	wsServer.AddRouter(common.MsgIdPing, &router.PingRouter{})
	wsServer.AddRouter(common.MsgIdLogin, &router.LoginRouter{GW: gw})
	wsServer.AddRouter(common.MsgIdChat, &router.ChatRouter{GW: gw})
}

func initBackendSvr(gw *router.GatewayRef) {
	initChatSvr(gw)
}

func initChatSvr(gw *router.GatewayRef) {
	csCfgs := conf.GlobalConfig.Services["chatsvr"]
	if len(csCfgs) == 0 {
		panic("no ChatSvr configured")
	}
	// init chat svr
	gw.ConnectBackend("chatsvr", csCfgs,
		func(conns []ziface.IConnection) router.BackendPool {
			return router.NewChatSvrPool(conns)
		},
		[]router.BackendRouterConfig{
			{MsgID: common.MsgIdLoginRsp, Router: &router.LoginRspRouter{GW: gw}},
			{MsgID: common.MsgIdBroadcast, Router: &router.BroadcastRouter{GW: gw}},
		},
	)
}

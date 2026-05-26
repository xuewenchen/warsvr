package main

import (
	"cardwar/apps/gateway/internal/router"
	"cardwar/common"
	"cardwar/conf"
	"flag"
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

	gw := &router.GatewayRef{
		Registry:    common.NewRegistry(),
		PlayerConns: &sync.Map{},
	}

	gw.Dial("chatsvr",
		[]common.BackendRouterConfig{
			{MsgID: common.MsgIdLoginRsp, Router: &router.LoginRspRouter{GW: gw}},
			{MsgID: common.MsgIdBroadcast, Router: &router.BroadcastRouter{GW: gw}},
		},
		common.HashRoute,
	)

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

	wsServer.AddRouter(common.MsgIdPing, &router.PingRouter{})
	wsServer.AddRouter(common.MsgIdLogin, &router.LoginRouter{GW: gw})
	wsServer.AddRouter(common.MsgIdChat, &router.ChatRouter{GW: gw})
}

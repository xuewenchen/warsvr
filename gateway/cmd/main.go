package main

import (
	"cardwar/common"
	"cardwar/conf"
	"cardwar/gateway/internal/router"
	"flag"
	"strconv"
	"strings"
	"sync"

	"github.com/aceld/zinx/zconf"
	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/zlog"
	"github.com/aceld/zinx/znet"
)

func parseHostPort(addr string) (string, int) {
	parts := strings.Split(addr, ":")
	if len(parts) != 2 {
		panic("invalid address: " + addr)
	}
	port, err := strconv.Atoi(parts[1])
	if err != nil {
		panic("invalid port: " + addr)
	}
	return parts[0], port
}

func main() {
	configPath := flag.String("conf", "config.yml", "path to config file")
	flag.Parse()

	if err := conf.Load(*configPath); err != nil {
		panic(err)
	}

	gwCfg := conf.GlobalConfig.Services.Gateway[0]
	csCfg := conf.GlobalConfig.Services.ChatSvr[0]

	gw := &router.GatewayRef{
		PlayerConns: &sync.Map{},
	}

	// TCP client → ChatSvr
	tcpReady := make(chan struct{})
	csHost, csPort := parseHostPort(csCfg.Listen)
	tcpClient := znet.NewClient(csHost, csPort)
	tcpClient.SetOnConnStart(func(conn ziface.IConnection) {
		gw.TCPConn = conn
		close(tcpReady)
		zlog.Ins().InfoF("Gateway connected to ChatSvr: %s", conn.RemoteAddr())
	})
	tcpClient.SetOnConnStop(func(conn ziface.IConnection) {
		gw.TCPConn = nil
		zlog.Ins().InfoF("Gateway disconnected from ChatSvr")
	})
	tcpClient.AddRouter(common.MsgIdLoginRsp, &router.LoginRspRouter{GW: gw})
	tcpClient.AddRouter(common.MsgIdBroadcast, &router.BroadcastRouter{GW: gw})

	// WS+TCP server for clients
	_, wsPort := parseHostPort(gwCfg.WSListen)
	tcpHost, tcpPort := parseHostPort(gwCfg.TCPListen)

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

	// Start TCP client first, wait for connection
	tcpClient.Start()
	<-tcpReady

	wsServer.Serve()
}

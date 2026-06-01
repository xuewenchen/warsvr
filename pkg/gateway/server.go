package gateway

import (
	"cardwar/pkg"
	"cardwar/pkg/auth"
	"cardwar/pkg/conf"
	"cardwar/pkg/connkey"
	"cardwar/pkg/corouter"
	"cardwar/protocol"
	"net/http"
	"sync"

	"github.com/aceld/zinx/zconf"
	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/zlog"
	"github.com/aceld/zinx/znet"
)

func initWebSocket(gw *GatewayServer, gwID string) {
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

	wsServer.SetOnConnStart(func(conn ziface.IConnection) {
		addr := conn.RemoteAddr().String()
		val, ok := pendingAuths.LoadAndDelete(addr)
		if !ok {
			zlog.Ins().ErrorF("Gateway: unauthenticated connection from %s, closing", addr)
			conn.Stop()
			return
		}
		playerID := val.(int64)
		conn.SetProperty(connkey.PropPlayerID, playerID)
		gw.PlayerConns.Store(playerID, conn.GetConnID())
		zlog.Ins().InfoF("Client connected: connID=%d, player=%d, addr=%s", conn.GetConnID(), playerID, addr)

		gw.CheckReconnect(playerID, conn)
	})

	wsServer.SetOnConnStop(func(conn ziface.IConnection) {
		if pidVal, err := conn.GetProperty(connkey.PropPlayerID); err == nil {
			if pid, ok := pidVal.(int64); ok {
				gw.PlayerConns.Delete(pid)
				gw.MarkDisconnected(pid)
			}
		}
		zlog.Ins().InfoF("Client disconnected: connID=%d", conn.GetConnID())
	})

	// 增加ping router
	wsServer.AddRouter(protocol.MsgIdPing, &corouter.PingRouter{})

	gw.InitForward(&ForwardRouter{GW: gw})
}

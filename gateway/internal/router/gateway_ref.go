package router

import (
	"cardwar/conf"
	"sync"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/zlog"
	"github.com/aceld/zinx/znet"
)

type GatewayRef struct {
	Server      ziface.IServer
	Backends    map[string]BackendPool
	PlayerConns *sync.Map // playerID → connID (uint64)
}

func (gw *GatewayRef) RouteTo(backend string, key string) ziface.IConnection {
	return gw.Backends[backend].Route(key)
}

// ConnectBackend connects to all server instances of a backend type and stores the pool.
func (gw *GatewayRef) ConnectBackend(name string, servers []conf.ServerNode, poolFactory func(conns []ziface.IConnection) BackendPool, routers []BackendRouterConfig) {
	if gw.Backends == nil {
		gw.Backends = make(map[string]BackendPool)
	}

	var wg sync.WaitGroup
	conns := make([]ziface.IConnection, len(servers))
	var bp *BaseBackendPool

	for i, svr := range servers {
		idx := i
		srv := svr

		host, port := conf.ParseHostPort(srv.Listen)
		tcpClient := znet.NewClient(host, port)

		tcpClient.SetOnConnStart(func(conn ziface.IConnection) {
			conns[idx] = conn
			zlog.Ins().InfoF("Gateway connected to %s[%s]: %s", name, srv.ID, conn.RemoteAddr())
			wg.Done()
		})
		tcpClient.SetOnConnStop(func(conn ziface.IConnection) {
			conns[idx] = nil
			zlog.Ins().InfoF("Gateway disconnected from %s[%s]", name, srv.ID)
			if bp != nil {
				bp.onDisconnect(idx)
			}
		})

		for _, rc := range routers {
			tcpClient.AddRouter(rc.MsgID, rc.Router)
		}

		wg.Add(1)
		tcpClient.Start()
	}

	wg.Wait()
	pool := poolFactory(conns)
	if hb, ok := pool.(interface{ getBase() *BaseBackendPool }); ok {
		bp = hb.getBase()
	}
	gw.Backends[name] = pool
}

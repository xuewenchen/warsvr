package router

import (
	"sync"

	"github.com/aceld/zinx/ziface"
)

type GatewayRef struct {
	Server         ziface.IServer
	ChatSvrTCPConn ziface.IConnection
	PlayerConns    *sync.Map // playerID → connID (uint64)
}

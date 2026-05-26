package router

import (
	"sync"

	"github.com/aceld/zinx/ziface"
)

type GatewayRef struct {
	Server      ziface.IServer
	TCPConn     ziface.IConnection
	PlayerConns *sync.Map // playerID → connID (uint64)
}

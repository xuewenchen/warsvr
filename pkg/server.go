package pkg

import (
	"cardwar/pkg/corouter"
	"cardwar/protocol"

	"github.com/aceld/zinx/zconf"
	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
)

// NewServer creates a Zinx server with common backend routers auto-registered
// (PingRouter, ServiceIdentityRouter). Backend services use this instead of raw
// znet.NewUserConfServer so they never need to remember manual router setup.
func NewServer(cfg *zconf.Config) ziface.IServer {
	s := znet.NewUserConfServer(cfg)
	s.AddRouter(protocol.MsgIdPing, &corouter.PingRouter{})
	s.AddRouter(protocol.MsgIdServiceIdentity, &corouter.ServiceIdentityRouter{})
	return s
}

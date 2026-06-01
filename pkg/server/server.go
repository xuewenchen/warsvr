package server

import (
	"cardwar/pkg/corouter"
	"cardwar/protocol"

	"github.com/aceld/zinx/zconf"
	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
)

// New creates a Zinx server with common backend routers auto-registered
// (PingRouter, ServiceIdentityRouter). Backend services use this instead of raw
// znet.NewUserConfServer so they never need to remember manual router setup.
// service is this backend's name (e.g. "chatsvr").
// forwardMsgIDs are msgIDs this backend handles (forward from clients),
// sendMsgIDs are msgIDs this backend may send (response/push to clients).
// These are announced via ServiceHello when a caller connects, enabling
// the caller to dynamically register forward/response routers.
func New(cfg *zconf.Config, service string, forwardMsgIDs, sendMsgIDs []uint32) ziface.IServer {
	s := znet.NewUserConfServer(cfg)
	s.AddRouter(protocol.MsgIdPing, &corouter.PingRouter{})
	s.AddRouter(protocol.MsgIdServiceIdentity, &corouter.ServiceIdentityRouter{
		Service:       service,
		ForwardMsgIDs: forwardMsgIDs,
		SendMsgIDs:    sendMsgIDs,
	})
	return s
}

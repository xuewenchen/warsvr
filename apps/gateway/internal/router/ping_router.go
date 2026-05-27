package router

import (
	"cardwar/protocol"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
)

type PingRouter struct {
	znet.BaseRouter
}

func (r *PingRouter) Handle(request ziface.IRequest) {
	request.GetConnection().SendMsg(protocol.MsgIdPong, []byte("pong-server"))
}

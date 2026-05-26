package router

import (
	"cardwar/common"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/zlog"
	"github.com/aceld/zinx/znet"
)

// PingRouter MsgIdPing路由
type PingRouter struct {
	znet.BaseRouter
}

// Ping Handle
func (this *PingRouter) Handle(request ziface.IRequest) {

	zlog.Ins().DebugF("Call PingRouter Handle")
	// Read the data from the client first, then send back "ping...ping...ping".
	zlog.Ins().DebugF("recv from client : msgId=%d, data=%+v, len=%d", request.GetMsgID(), string(request.GetData()), len(request.GetData()))

	err := request.GetConnection().SendMsg(common.MsgIdPong, []byte("pong-server"))
	if err != nil {
		zlog.Error(err)
	}
}

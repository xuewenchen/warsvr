package router

import (
	"cardwar/common"
	"encoding/json"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/zlog"
	"github.com/aceld/zinx/znet"
)

type LoginRouter struct {
	znet.BaseRouter
	GW *GatewayRef
}

func (r *LoginRouter) Handle(request ziface.IRequest) {
	if r.GW.TCPConn == nil {
		zlog.Ins().ErrorF("LoginRouter: no connection to ChatSvr")
		return
	}

	env := common.Envelope{
		ConnID: request.GetConnection().GetConnID(),
		Data:   request.GetData(),
	}
	envData, _ := json.Marshal(env)

	r.GW.TCPConn.SendMsg(common.MsgIdLogin, envData)
}

package router

import (
	"cardwar/common"
	"encoding/json"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/zlog"
	"github.com/aceld/zinx/znet"
)

type ChatRouter struct {
	znet.BaseRouter
	GW *GatewayRef
}

func (r *ChatRouter) Handle(request ziface.IRequest) {
	if r.GW.ChatSvrTCPConn == nil {
		zlog.Ins().ErrorF("ChatRouter: no connection to ChatSvr")
		return
	}

	env := common.Envelope{
		ConnID: request.GetConnection().GetConnID(),
		Data:   request.GetData(),
	}
	envData, _ := json.Marshal(env)

	r.GW.ChatSvrTCPConn.SendMsg(common.MsgIdChat, envData)
}

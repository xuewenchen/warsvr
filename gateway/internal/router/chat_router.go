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
	var msg common.ChatMsg
	if err := json.Unmarshal(request.GetData(), &msg); err != nil {
		zlog.Error(err)
		return
	}

	env := common.Envelope{
		ConnID: request.GetConnection().GetConnID(),
		Data:   request.GetData(),
	}
	envData, _ := json.Marshal(env)

	conn := r.GW.ChatSvrPool.Route(msg.PlayerID)
	conn.SendMsg(common.MsgIdChat, envData)
}

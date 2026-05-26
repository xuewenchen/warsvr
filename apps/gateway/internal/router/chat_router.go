package router

import (
	"cardwar/protocol"
	"cardwar/protocol/pb"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/zlog"
	"github.com/aceld/zinx/znet"
	"google.golang.org/protobuf/proto"
)

type ChatRouter struct {
	znet.BaseRouter
	GW *GatewayRef
}

func (r *ChatRouter) Handle(request ziface.IRequest) {
	var msg pb.ChatReq
	if err := proto.Unmarshal(request.GetData(), &msg); err != nil {
		zlog.Error(err)
		return
	}

	env := &pb.Envelope{
		ConnId: request.GetConnection().GetConnID(),
		Data:   request.GetData(),
	}
	envData, _ := proto.Marshal(env)

	conn := r.GW.RouteTo("chatsvr", msg.PlayerId)
	if conn == nil {
		zlog.Ins().ErrorF("ChatRouter: no healthy chatsvr backend")
		return
	}
	conn.SendMsg(protocol.MsgIdChat, envData)
}

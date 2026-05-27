package router

import (
	"cardwar/protocol"
	"cardwar/protocol/pb"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/zlog"
	"github.com/aceld/zinx/znet"
	"google.golang.org/protobuf/proto"
)

type ChatRouter struct {
	znet.BaseRouter
}

func (r *ChatRouter) Handle(request ziface.IRequest) {
	var env pb.Envelope
	if err := proto.Unmarshal(request.GetData(), &env); err != nil {
		zlog.Error(err)
		return
	}

	var chatReq pb.ChatReq
	if err := proto.Unmarshal(env.Data, &chatReq); err != nil {
		zlog.Error(err)
		return
	}

	bcMsg := &pb.BroadcastPush{
		PlayerId:  chatReq.PlayerId,
		Content:   chatReq.Content,
		Timestamp: time.Now().Unix(),
	}
	bcData, _ := proto.Marshal(bcMsg)

	bcEnv := &pb.Envelope{ConnId: 0, Data: bcData}
	bcEnvData, _ := proto.Marshal(bcEnv)

	request.GetConnection().SendMsg(protocol.MsgIdBroadcast, bcEnvData)

	zlog.Ins().InfoF("Broadcast from %s: %s", chatReq.PlayerId, chatReq.Content)
}

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

	senderPID := env.ConnTags["player_id"]

	push := &pb.ChatResp{
		SenderPlayerId: senderPID,
		Content:        chatReq.Content,
		Timestamp:      time.Now().Unix(),
		TargetPlayerId: chatReq.TargetPlayerId,
	}
	pushData, _ := proto.Marshal(push)

	var pushEnv *pb.Envelope
	if chatReq.TargetPlayerId != "" {
		// Private: route to target player via Gateway
		pushEnv = &pb.Envelope{
			ConnId: 0,
			Data:   pushData,
			ConnTags: map[string]string{
				"target_player_id": chatReq.TargetPlayerId,
			},
		}
		zlog.Ins().InfoF("Chat private: %s -> %s: %s", senderPID, chatReq.TargetPlayerId, chatReq.Content)
	} else {
		// Global: broadcast to all
		pushEnv = &pb.Envelope{ConnId: 0, Data: pushData}
		zlog.Ins().InfoF("Chat global: %s: %s", senderPID, chatReq.Content)
	}

	pushEnvData, _ := proto.Marshal(pushEnv)
	request.GetConnection().SendMsg(protocol.MsgIdChatResp, pushEnvData)
}

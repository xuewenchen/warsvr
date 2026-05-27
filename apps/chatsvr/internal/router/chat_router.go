package router

import (
	"cardwar/pkg"
	"cardwar/protocol"
	"cardwar/protocol/pb"
	"strconv"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/zlog"
	"github.com/aceld/zinx/znet"
	"google.golang.org/protobuf/proto"
)

type ChatRouter struct {
	znet.BaseRouter
	BC *pkg.Broadcaster
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

	senderPID, _ := strconv.ParseInt(env.ConnTags["player_id"], 10, 64)

	push := &pb.ChatResp{
		SenderPlayerId: senderPID,
		Content:        chatReq.Content,
		Timestamp:      time.Now().Unix(),
		TargetPlayerId: chatReq.TargetPlayerId,
	}
	pushData, _ := proto.Marshal(push)

	if chatReq.TargetPlayerId != 0 {
		r.BC.ToPlayer(protocol.MsgIdChatResp, chatReq.TargetPlayerId, pushData)
		r.BC.ToConn(protocol.MsgIdChatResp, env.ConnId, pushData, request.GetConnection())
	} else {
		r.BC.ToAll(protocol.MsgIdChatResp, pushData)
	}
}

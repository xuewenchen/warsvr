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
	Server ziface.IServer
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

	if chatReq.TargetPlayerId != "" {
		r.sendPrivate(request.GetConnection(), env.ConnId, chatReq.TargetPlayerId, pushData)
	} else {
		r.sendGlobal(pushData)
	}
}

func (r *ChatRouter) sendGlobal(pushData []byte) {
	envData, _ := proto.Marshal(&pb.Envelope{ConnId: 0, Data: pushData})
	r.Server.GetConnMgr().Range(func(connID uint64, conn ziface.IConnection, extra interface{}) error {
		conn.SendMsg(protocol.MsgIdChatResp, envData)
		return nil
	}, nil)
	zlog.Ins().InfoF("Chat global broadcast to %d gateways", r.Server.GetConnMgr().Len())
}

func (r *ChatRouter) sendPrivate(reqConn ziface.IConnection, senderConnID uint64, targetPID string, pushData []byte) {
	// Delivery to target: broadcast to ALL Gateways, each Gateway's ResponseRouter checks PlayerConns
	targetEnv := &pb.Envelope{
		ConnId:   0,
		Data:     pushData,
		ConnTags: map[string]string{"target_player_id": targetPID},
	}
	targetEnvData, _ := proto.Marshal(targetEnv)
	r.Server.GetConnMgr().Range(func(connID uint64, conn ziface.IConnection, extra interface{}) error {
		conn.SendMsg(protocol.MsgIdChatResp, targetEnvData)
		return nil
	}, nil)

	// Confirmation to sender on the original connection
	senderEnvData, _ := proto.Marshal(&pb.Envelope{ConnId: senderConnID, Data: pushData})
	reqConn.SendMsg(protocol.MsgIdChatResp, senderEnvData)

	zlog.Ins().InfoF("Chat private to %s", targetPID)
}

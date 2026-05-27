package router

import (
	"cardwar/protocol/pb"
	"fmt"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
	"google.golang.org/protobuf/proto"
)

type ChatPushRouter struct {
	znet.BaseRouter
}

func (r *ChatPushRouter) Handle(request ziface.IRequest) {
	var msg pb.ChatPush
	if err := proto.Unmarshal(request.GetData(), &msg); err != nil {
		fmt.Println("ChatPush parse error:", err)
		return
	}
	if msg.TargetPlayerId != "" {
		fmt.Printf("[Private] %s -> %s: %s\n", msg.SenderPlayerId, msg.TargetPlayerId, msg.Content)
	} else {
		fmt.Printf("[Global] %s: %s\n", msg.SenderPlayerId, msg.Content)
	}
}

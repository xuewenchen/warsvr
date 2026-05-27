package router

import (
	"cardwar/protocol/pb"
	"fmt"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
	"google.golang.org/protobuf/proto"
)

type ChatRespRouter struct {
	znet.BaseRouter
}

func (r *ChatRespRouter) Handle(request ziface.IRequest) {
	var msg pb.ChatResp
	if err := proto.Unmarshal(request.GetData(), &msg); err != nil {
		fmt.Println("ChatResp parse error:", err)
		return
	}
	if msg.TargetPlayerId != 0 {
		fmt.Printf("[Private] %d -> %d: %s\n", msg.SenderPlayerId, msg.TargetPlayerId, msg.Content)
	} else {
		fmt.Printf("[Global] %d: %s\n", msg.SenderPlayerId, msg.Content)
	}
}

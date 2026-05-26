package router

import (
	"cardwar/protocol/pb"
	"fmt"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
	"google.golang.org/protobuf/proto"
)

type BroadcastRouter struct {
	znet.BaseRouter
}

func (r *BroadcastRouter) Handle(request ziface.IRequest) {
	var msg pb.BroadcastPush
	if err := proto.Unmarshal(request.GetData(), &msg); err != nil {
		fmt.Println("Broadcast parse error:", err)
		return
	}
	fmt.Printf("[Broadcast] %s: %s\n", msg.PlayerId, msg.Content)
}

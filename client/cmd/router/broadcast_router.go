package router

import (
	"cardwar/common"
	"encoding/json"
	"fmt"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
)

type BroadcastRouter struct {
	znet.BaseRouter
}

func (r *BroadcastRouter) Handle(request ziface.IRequest) {
	var msg common.BroadcastMsg
	if err := json.Unmarshal(request.GetData(), &msg); err != nil {
		fmt.Println("Broadcast parse error:", err)
		return
	}
	fmt.Printf("[Broadcast] %s: %s\n", msg.PlayerID, msg.Content)
}

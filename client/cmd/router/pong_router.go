package router

import (
	"fmt"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
)

// PongRouter pong test 自定义路由
type PongRouter struct {
	znet.BaseRouter
}

// Handle Pong Handle
func (this *PongRouter) Handle(request ziface.IRequest) {
	fmt.Println("Call PongRouter Handle")
}

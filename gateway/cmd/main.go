package main

import (
	"cardwar/common"
	"cardwar/gateway/router"

	"github.com/aceld/zinx/znet"
)

// PingRouter MsgId=1的路由

func main() {

	//1 创建一个server服务
	s := znet.NewServer()

	//2 配置路由
	s.AddRouter(common.MsgIdPing, &router.PingRouter{})

	//3 启动服务
	s.Serve()
}

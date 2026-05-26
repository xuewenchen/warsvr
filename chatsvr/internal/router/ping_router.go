package router

import (
	"cardwar/common"
	"fmt"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
)

// PingRouter MsgIdPing路由
type PingRouter struct {
	znet.BaseRouter
}

// Ping Handle MsgIdPing的路由处理方法
func (r *PingRouter) PreHandle(request ziface.IRequest) {
	//读取客户端的数据
	fmt.Println("PreHandle: recv from client : msgId=", request.GetMsgID(), ", data=", string(request.GetData()))
}

// Ping Handle MsgIdPing的路由处理方法
func (r *PingRouter) Handle(request ziface.IRequest) {
	//读取客户端的数据
	fmt.Println("Handle: recv from client : msgId=", request.GetMsgID(), ", data=", string(request.GetData()))
	request.GetConnection().SendMsg(common.MsgIdPong, []byte("pong...pong...pong...[FromServer]"))
}

// Ping Handle MsgIdPing的路由处理方法
func (r *PingRouter) PostHandle(request ziface.IRequest) {
	//读取客户端的数据
	fmt.Println("PostHandle: recv from client : msgId=", request.GetMsgID(), ", data=", string(request.GetData()))
}

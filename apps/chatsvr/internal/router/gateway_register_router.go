package router

import (
	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
)

// GatewayRegisterRouter tags incoming connections from Gateways.
// The Gateway sends MsgIdGatewayRegister immediately on Dial connect.
type GatewayRegisterRouter struct {
	znet.BaseRouter
}

func (r *GatewayRegisterRouter) Handle(request ziface.IRequest) {
	request.GetConnection().SetProperty("conn_type", "gateway")
}

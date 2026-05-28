package router

import (
	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
)

type GatewayRegisterRouter struct {
	znet.BaseRouter
}

func (r *GatewayRegisterRouter) Handle(request ziface.IRequest) {
	request.GetConnection().SetProperty("conn_type", "gateway")
}

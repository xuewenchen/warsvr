package corouter

import (
	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
)

type GatewayRegisterRouter struct {
	znet.BaseRouter
}

// 设置链接类型是gateway
func (r *GatewayRegisterRouter) Handle(request ziface.IRequest) {
	request.GetConnection().SetProperty("conn_type", "gateway")
}

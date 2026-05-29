package corouter

import (
	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
)

// ServiceIdentityRouter handles service identity messages sent by dialing services.
// It sets the conn_type property on the connection to the caller's identity string
// (e.g. "gateway"), which downstream code like NewGateWayBroadcaster uses for filtering.
type ServiceIdentityRouter struct {
	znet.BaseRouter
}

func (r *ServiceIdentityRouter) Handle(request ziface.IRequest) {
	identity := string(request.GetData())
	request.GetConnection().SetProperty("conn_type", identity)
}

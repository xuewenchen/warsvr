package gateway

import (
	"cardwar/pkg"
	"cardwar/protocol/pb"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/zlog"
	"github.com/aceld/zinx/znet"
	"google.golang.org/protobuf/proto"
)

// ServiceHelloRouter handles ServiceHello messages sent by backends
// in response to the gateway's identity announcement. Each backend lists
// the msgIDs it handles (forward) and sends (response/push).
// The gateway dynamically registers ForwardRouter on the WebSocket server
// for forward msgIDs and ResponseRouter on the backend connection for send msgIDs.
type ServiceHelloRouter struct {
	znet.BaseRouter
	GW *GatewayServer
}

func (r *ServiceHelloRouter) Handle(request ziface.IRequest) {
	var hello pb.ServiceHello
	if err := proto.Unmarshal(request.GetData(), &hello); err != nil {
		zlog.Error(err)
		return
	}

	gw := r.GW
	gw.mu.Lock()
	cfg, ok := gw.backendCfgs[hello.Service]
	if !ok {
		gw.mu.Unlock()
		zlog.Ins().ErrorF("Gateway: hello from unknown backend %s", hello.Service)
		return
	}

	// Register forward msgIDs on the WebSocket server
	for _, msgID := range hello.MsgIds {
		if gw.fwdRegistered[msgID] {
			continue
		}
		gw.fwdRegistered[msgID] = true
		gw.routes[msgID] = &BackendRouteInfo{
			Backend:   hello.Service,
			RouteKey:  cfg.RouteKey,
			RouteType: cfg.RouteType,
		}
		gw.mu.Unlock()
		gw.Server.AddRouter(msgID, gw.FwdRouter)
		gw.mu.Lock()
	}
	gw.mu.Unlock()

	zlog.Ins().InfoF("Gateway: hello from %s — registered %d forward, %d send msgIDs",
		hello.Service, len(hello.MsgIds), len(hello.SendMsgIds))

	// Register response msgIDs on the backend connection
	conn := request.GetConnection()
	pool := gw.backendPool(hello.Service)
	if pool == nil {
		return
	}

	// 后端服务的send msgIDs由网关统一注册ResponseRouter处理
	for _, msgID := range hello.SendMsgIds {
		pool.AddConnectionRouter(conn, msgID, gw.RspRouter)
	}
}

// backendPool returns the BackendPool for the named service, or nil.
func (gw *GatewayServer) backendPool(service string) pkg.BackendPool {
	return gw.Registry.Pool(service)
}

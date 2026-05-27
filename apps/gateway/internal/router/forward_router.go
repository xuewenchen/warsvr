package router

import (
	"cardwar/protocol/pb"
	"fmt"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/zlog"
	"github.com/aceld/zinx/znet"
	"google.golang.org/protobuf/proto"
)

// ForwardRouter is a generic router that forwards client messages to the configured backend.
// It does not parse the message body; it wraps the raw bytes in an Envelope and routes
// based on connection metadata (connId or playerId).
type ForwardRouter struct {
	znet.BaseRouter
	GW *GatewayRef
}

func (r *ForwardRouter) Handle(request ziface.IRequest) {
	msgID := request.GetMsgID()
	route := r.GW.RouteFor(msgID)
	if route == nil {
		zlog.Ins().ErrorF("ForwardRouter: no route for msgID=%d", msgID)
		return
	}

	routeKey := r.resolveRouteKey(request.GetConnection(), route)
	if routeKey == "" {
		zlog.Ins().ErrorF("ForwardRouter: empty route key for msgID=%d", msgID)
		return
	}

	env := &pb.Envelope{
		ConnId: request.GetConnection().GetConnID(),
		Data:   request.GetData(),
	}
	envData, _ := proto.Marshal(env)

	conn := r.GW.RouteTo(route.Backend, routeKey)
	if conn == nil {
		zlog.Ins().ErrorF("ForwardRouter: no healthy backend for %s msgID=%d", route.Backend, msgID)
		return
	}
	conn.SendMsg(msgID, envData)
}

func (r *ForwardRouter) resolveRouteKey(conn ziface.IConnection, route *BackendRouteInfo) string {
	if route.RouteKey == "playerId" {
		if pid, err := conn.GetProperty("playerId"); err == nil {
			return pid.(string)
		}
	}
	// Fall back to connId when playerId is not yet set (e.g., login message)
	return fmt.Sprintf("%d", conn.GetConnID())
}

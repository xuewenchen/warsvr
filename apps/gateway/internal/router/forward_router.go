package router

import (
	"cardwar/protocol"
	"cardwar/protocol/pb"
	"fmt"
	"strconv"
	"time"

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

	// 封包
	env := &pb.Envelope{
		ConnId: request.GetConnection().GetConnID(),
		Data:   request.GetData(),
	}
	// 封包塞入额外字段
	if pid, err := request.GetConnection().GetProperty("playerId"); err == nil {
		env.ConnTags = map[string]string{"player_id": strconv.FormatInt(pid.(int64), 10)}
	}
	envData, _ := proto.Marshal(env)

	conn := r.GW.RouteTo(route.Backend, routeKey)
	if conn == nil {
		zlog.Ins().ErrorF("ForwardRouter: no healthy backend for %s msgID=%d", route.Backend, msgID)
		r.sendError(request, msgID)
		return
	}
	conn.SendMsg(msgID, envData)
}

func (r *ForwardRouter) sendError(request ziface.IRequest, reqMsgID uint32) {
	// Map request msgID to response msgID for the error response.
	// Convention: application request msgIDs have a +1 response msgID.
	var respMsgID uint32
	switch reqMsgID {
	case protocol.MsgIdChatReq:
		respMsgID = protocol.MsgIdChatResp
	default:
		respMsgID = reqMsgID + 1
	}

	errResp := &pb.ChatResp{
		SenderPlayerId: -1,
		Content:        "service unavailable: backend offline",
		Timestamp:      time.Now().Unix(),
	}
	data, _ := proto.Marshal(errResp)
	request.GetConnection().SendMsg(respMsgID, data)
}

func (r *ForwardRouter) resolveRouteKey(conn ziface.IConnection, route *BackendRouteInfo) string {
	key := route.RouteKey
	if key == "" || key == "connId" {
		return fmt.Sprintf("%d", conn.GetConnID())
	}
	// Try the named property: playerId, roomId, or any custom key
	if val, err := conn.GetProperty(key); err == nil {
		return fmt.Sprintf("%v", val)
	}
	return fmt.Sprintf("%d", conn.GetConnID())
}

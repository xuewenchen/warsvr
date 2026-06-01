package gateway

import (
	"cardwar/pkg/connkey"
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
	GW *GatewayServer
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
	if pid, err := request.GetConnection().GetProperty(connkey.PropPlayerID); err == nil {
		env.ConnTags = map[string]string{connkey.TagPlayerID: strconv.FormatInt(pid.(int64), 10)}
	}
	envData, _ := proto.Marshal(env)

	conn := r.GW.RouteTo(route.Backend, routeKey)
	if conn == nil {
		zlog.Ins().ErrorF("ForwardRouter: no healthy backend for %s msgID=%d", route.Backend, msgID)
		r.sendError(request.GetConnection(), msgID)
		return
	}
	conn.SendMsg(msgID, envData)
}

func (r *ForwardRouter) sendError(conn ziface.IConnection, reqMsgID uint32) {
	errMsg := "service unavailable: backend offline"

	switch reqMsgID {
	case protocol.MsgIdChatReq:
		data, _ := proto.Marshal(&pb.ChatResp{
			SenderPlayerId: -1,
			Content:        errMsg,
			Timestamp:      time.Now().Unix(),
		})
		conn.SendMsg(protocol.MsgIdChatResp, data)

	case protocol.MsgIdMatchEnterReq:
		data, _ := proto.Marshal(&pb.MatchEnterResp{Status: "error"})
		conn.SendMsg(protocol.MsgIdMatchEnterResp, data)

	case protocol.MsgIdMatchAllocateReq:
		data, _ := proto.Marshal(&pb.MatchAllocateResp{Error: errMsg})
		conn.SendMsg(protocol.MsgIdMatchAllocateResp, data)

	case protocol.MsgIdMatchQueryReq:
		data, _ := proto.Marshal(&pb.MatchQueryResp{Found: false})
		conn.SendMsg(protocol.MsgIdMatchQueryResp, data)

	case protocol.MsgIdRoomJoinReq:
		data, _ := proto.Marshal(&pb.RoomJoinResp{Success: false, Error: errMsg})
		conn.SendMsg(protocol.MsgIdRoomJoinResp, data)

	case protocol.MsgIdRoomLeaveReq:
		data, _ := proto.Marshal(&pb.RoomLeaveResp{Success: false, Error: errMsg})
		conn.SendMsg(protocol.MsgIdRoomLeaveResp, data)

	default:
		// Unknown msgID — send empty body with reqMsgID+1 convention.
		conn.SendMsg(reqMsgID+1, nil)
	}
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

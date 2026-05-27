package router

import (
	"cardwar/protocol/pb"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/zlog"
	"github.com/aceld/zinx/znet"
	"google.golang.org/protobuf/proto"
)

// ResponseRouter is a generic router that unwraps Envelope responses from backends
// and forwards them to the correct client. It also applies conn_tags from the
// Envelope to the client connection's properties.
type ResponseRouter struct {
	znet.BaseRouter
	GW *GatewayRef
}

func (r *ResponseRouter) Handle(request ziface.IRequest) {
	msgID := request.GetMsgID()

	var env pb.Envelope
	if err := proto.Unmarshal(request.GetData(), &env); err != nil {
		zlog.Error(err)
		return
	}

	// Private routing: check target_player_id in conn_tags
	if targetPID := env.ConnTags["target_player_id"]; targetPID != "" {
		val, ok := r.GW.PlayerConns.Load(targetPID)
		if !ok {
			zlog.Ins().ErrorF("ResponseRouter: target player not connected: %s", targetPID)
			return
		}
		wsConn, err := r.GW.Server.GetConnMgr().Get(val.(uint64))
		if err != nil {
			zlog.Ins().ErrorF("ResponseRouter: target conn not found for player %s: %v", targetPID, err)
			return
		}
		wsConn.SendMsg(msgID, env.Data)
		return
	}

	if env.ConnId != 0 {
		wsConn, err := r.GW.Server.GetConnMgr().Get(env.ConnId)
		if err != nil {
			zlog.Ins().ErrorF("ResponseRouter: client conn not found: %d", env.ConnId)
			return
		}
		r.applyConnTags(wsConn, env.ConnTags)
		wsConn.SendMsg(msgID, env.Data)
		return
	}

	// conn_id=0 means broadcast to all clients
	r.GW.Server.GetConnMgr().Range(func(connID uint64, conn ziface.IConnection, extra interface{}) error {
		conn.SendMsg(msgID, env.Data)
		return nil
	}, nil)
}

func (r *ResponseRouter) applyConnTags(conn ziface.IConnection, tags map[string]string) {
	if len(tags) == 0 {
		return
	}
	for k, v := range tags {
		conn.SetProperty(k, v)
	}
	if pid, ok := tags["playerId"]; ok {
		r.GW.PlayerConns.Store(pid, conn.GetConnID())
	}
}

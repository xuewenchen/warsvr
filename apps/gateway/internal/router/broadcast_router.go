package router

import (
	"cardwar/protocol"
	"cardwar/protocol/pb"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/zlog"
	"github.com/aceld/zinx/znet"
	"google.golang.org/protobuf/proto"
)

type BroadcastRouter struct {
	znet.BaseRouter
	GW *GatewayRef
}

func (r *BroadcastRouter) Handle(request ziface.IRequest) {
	var env pb.Envelope
	if err := proto.Unmarshal(request.GetData(), &env); err != nil {
		zlog.Error(err)
		return
	}

	if env.ConnId == 0 {
		r.GW.Server.GetConnMgr().Range(func(connID uint64, conn ziface.IConnection, extra interface{}) error {
			conn.SendMsg(protocol.MsgIdBroadcast, env.Data)
			return nil
		}, nil)
	} else {
		wsConn, err := r.GW.Server.GetConnMgr().Get(env.ConnId)
		if err != nil {
			zlog.Ins().ErrorF("BroadcastRouter: client conn not found: %d", env.ConnId)
			return
		}
		wsConn.SendMsg(protocol.MsgIdBroadcast, env.Data)
	}
}

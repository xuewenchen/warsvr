package router

import (
	"cardwar/common"
	"encoding/json"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/zlog"
	"github.com/aceld/zinx/znet"
)

type BroadcastRouter struct {
	znet.BaseRouter
	GW *GatewayRef
}

func (r *BroadcastRouter) Handle(request ziface.IRequest) {
	var env common.Envelope
	if err := json.Unmarshal(request.GetData(), &env); err != nil {
		zlog.Error(err)
		return
	}

	if env.ConnID == 0 {
		r.GW.Server.GetConnMgr().Range(func(connID uint64, conn ziface.IConnection, extra interface{}) error {
			conn.SendMsg(common.MsgIdBroadcast, env.Data)
			return nil
		}, nil)
	} else {
		wsConn, err := r.GW.Server.GetConnMgr().Get(env.ConnID)
		if err != nil {
			zlog.Ins().ErrorF("BroadcastRouter: client conn not found: %d", env.ConnID)
			return
		}
		wsConn.SendMsg(common.MsgIdBroadcast, env.Data)
	}
}

package router

import (
	"cardwar/common"
	"encoding/json"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/zlog"
	"github.com/aceld/zinx/znet"
)

type LoginRspRouter struct {
	znet.BaseRouter
	GW *GatewayRef
}

func (r *LoginRspRouter) Handle(request ziface.IRequest) {
	var env common.Envelope
	if err := json.Unmarshal(request.GetData(), &env); err != nil {
		zlog.Error(err)
		return
	}

	wsConn, err := r.GW.Server.GetConnMgr().Get(env.ConnID)
	if err != nil {
		zlog.Ins().ErrorF("LoginRspRouter: client conn not found: %d", env.ConnID)
		return
	}

	var rsp common.LoginRspMsg
	if err := json.Unmarshal(env.Data, &rsp); err == nil && rsp.Success {
		wsConn.SetProperty("playerId", rsp.PlayerID)
		r.GW.PlayerConns.Store(rsp.PlayerID, env.ConnID)
	}

	wsConn.SendMsg(common.MsgIdLoginRsp, env.Data)
}

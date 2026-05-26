package router

import (
	"cardwar/protocol"
	"cardwar/protocol/pb"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/zlog"
	"github.com/aceld/zinx/znet"
	"google.golang.org/protobuf/proto"
)

type LoginRspRouter struct {
	znet.BaseRouter
	GW *GatewayRef
}

func (r *LoginRspRouter) Handle(request ziface.IRequest) {
	var env pb.Envelope
	if err := proto.Unmarshal(request.GetData(), &env); err != nil {
		zlog.Error(err)
		return
	}

	wsConn, err := r.GW.Server.GetConnMgr().Get(env.ConnId)
	if err != nil {
		zlog.Ins().ErrorF("LoginRspRouter: client conn not found: %d", env.ConnId)
		return
	}

	var rsp pb.LoginRsp
	if err := proto.Unmarshal(env.Data, &rsp); err == nil && rsp.Success {
		wsConn.SetProperty("playerId", rsp.PlayerId)
		r.GW.PlayerConns.Store(rsp.PlayerId, env.ConnId)
	}

	wsConn.SendMsg(protocol.MsgIdLoginRsp, env.Data)
}

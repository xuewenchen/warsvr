package router

import (
	"cardwar/protocol"
	"cardwar/protocol/pb"
	"sync"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/zlog"
	"github.com/aceld/zinx/znet"
	"google.golang.org/protobuf/proto"
)

var loggedInPlayers sync.Map

type LoginRouter struct {
	znet.BaseRouter
}

func (r *LoginRouter) Handle(request ziface.IRequest) {
	var env pb.Envelope
	if err := proto.Unmarshal(request.GetData(), &env); err != nil {
		zlog.Error(err)
		return
	}

	var loginReq pb.LoginReq
	if err := proto.Unmarshal(env.Data, &loginReq); err != nil {
		zlog.Error(err)
		return
	}

	if loginReq.PlayerId == "" {
		zlog.Ins().ErrorF("LoginRouter: empty player_id")
		return
	}

	loggedInPlayers.Store(loginReq.PlayerId, struct{}{})
	zlog.Ins().InfoF("Player logged in: %s", loginReq.PlayerId)

	rsp := &pb.LoginRsp{PlayerId: loginReq.PlayerId, Success: true, Message: "login ok"}
	rspData, _ := proto.Marshal(rsp)

	rspEnv := &pb.Envelope{ConnId: env.ConnId, Data: rspData}
	rspEnvData, _ := proto.Marshal(rspEnv)

	request.GetConnection().SendMsg(protocol.MsgIdLoginRsp, rspEnvData)
}

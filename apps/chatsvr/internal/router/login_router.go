package router

import (
	"cardwar/common"
	"encoding/json"
	"sync"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/zlog"
	"github.com/aceld/zinx/znet"
)

var loggedInPlayers sync.Map

type LoginRouter struct {
	znet.BaseRouter
}

func (r *LoginRouter) Handle(request ziface.IRequest) {
	var env common.Envelope
	if err := json.Unmarshal(request.GetData(), &env); err != nil {
		zlog.Error(err)
		return
	}

	var loginMsg common.LoginMsg
	if err := json.Unmarshal(env.Data, &loginMsg); err != nil {
		zlog.Error(err)
		return
	}

	if loginMsg.PlayerID == "" {
		zlog.Ins().ErrorF("LoginRouter: empty player_id")
		return
	}

	loggedInPlayers.Store(loginMsg.PlayerID, struct{}{})
	zlog.Ins().InfoF("Player logged in: %s", loginMsg.PlayerID)

	rsp := common.LoginRspMsg{PlayerID: loginMsg.PlayerID, Success: true, Message: "login ok"}
	rspData, _ := json.Marshal(rsp)

	rspEnv := common.Envelope{ConnID: env.ConnID, Data: rspData}
	rspEnvData, _ := json.Marshal(rspEnv)

	request.GetConnection().SendMsg(common.MsgIdLoginRsp, rspEnvData)
}

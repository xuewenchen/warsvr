package router

import (
	"cardwar/common"
	"encoding/json"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/zlog"
	"github.com/aceld/zinx/znet"
)

type ChatRouter struct {
	znet.BaseRouter
}

func (r *ChatRouter) Handle(request ziface.IRequest) {
	var env common.Envelope
	if err := json.Unmarshal(request.GetData(), &env); err != nil {
		zlog.Error(err)
		return
	}

	var chatMsg common.ChatMsg
	if err := json.Unmarshal(env.Data, &chatMsg); err != nil {
		zlog.Error(err)
		return
	}

	if _, ok := loggedInPlayers.Load(chatMsg.PlayerID); !ok {
		zlog.Ins().InfoF("ChatRouter: player not logged in: %s", chatMsg.PlayerID)
		return
	}

	bcMsg := common.BroadcastMsg{
		PlayerID:  chatMsg.PlayerID,
		Content:   chatMsg.Content,
		Timestamp: time.Now().Unix(),
	}
	bcData, _ := json.Marshal(bcMsg)

	bcEnv := common.Envelope{ConnID: 0, Data: bcData}
	bcEnvData, _ := json.Marshal(bcEnv)

	request.GetConnection().SendMsg(common.MsgIdBroadcast, bcEnvData)

	zlog.Ins().InfoF("Broadcast from %s: %s", chatMsg.PlayerID, chatMsg.Content)
}

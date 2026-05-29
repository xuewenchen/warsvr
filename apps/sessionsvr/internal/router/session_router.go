package router

import (
	"sync"
	"time"

	"cardwar/pkg"
	"cardwar/protocol"
	"cardwar/protocol/pb"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/zlog"
	"github.com/aceld/zinx/znet"
	"google.golang.org/protobuf/proto"
)

const SessionTTL = 120 * time.Second

type Session struct {
	PlayerID       int64
	GatewayID      string
	ConnTags       map[string]string
	DisconnectedAt int64 // unix timestamp, 0 = connected
}

var sessions sync.Map // playerId(int64) → *Session

type SessionRouter struct {
	znet.BaseRouter
	Reg *pkg.Registry
}

func (r *SessionRouter) Handle(request ziface.IRequest) {
	switch request.GetMsgID() {
	case protocol.MsgIdSessionSave:
		r.handleSave(request)
	case protocol.MsgIdSessionGet:
		r.handleGet(request)
	case protocol.MsgIdSessionDisconnect:
		r.handleDisconnect(request)
	case protocol.MsgIdSessionReconnect:
		r.handleReconnect(request)
	}
}

func (r *SessionRouter) handleSave(request ziface.IRequest) {
	var data pb.SessionData
	if err := proto.Unmarshal(request.GetData(), &data); err != nil {
		zlog.Error(err)
		return
	}
	s := &Session{
		PlayerID:  data.PlayerId,
		GatewayID: data.GatewayId,
		ConnTags:  data.ConnTags,
	}
	sessions.Store(data.PlayerId, s)
}

func (r *SessionRouter) handleGet(request ziface.IRequest) {
	var data pb.SessionData
	if err := proto.Unmarshal(request.GetData(), &data); err != nil {
		zlog.Error(err)
		return
	}
	v, ok := sessions.Load(data.PlayerId)
	if !ok {
		return // no session → no response (caller treats nil as expired)
	}
	s := v.(*Session)
	resp, _ := proto.Marshal(&pb.SessionData{
		PlayerId:       s.PlayerID,
		GatewayId:      s.GatewayID,
		ConnTags:       s.ConnTags,
		DisconnectedAt: s.DisconnectedAt,
	})
	request.GetConnection().SendMsg(protocol.MsgIdSessionGet, resp)
}

func (r *SessionRouter) handleDisconnect(request ziface.IRequest) {
	var data pb.SessionData
	if err := proto.Unmarshal(request.GetData(), &data); err != nil {
		zlog.Error(err)
		return
	}
	v, ok := sessions.Load(data.PlayerId)
	if !ok {
		v = &Session{PlayerID: data.PlayerId, GatewayID: data.GatewayId, ConnTags: data.ConnTags}
		sessions.Store(data.PlayerId, v)
	}
	s := v.(*Session)
	s.DisconnectedAt = time.Now().Unix()
	zlog.Ins().InfoF("SessionSvr: player %d disconnected (gateway=%s)", s.PlayerID, s.GatewayID)
}

func (r *SessionRouter) handleReconnect(request ziface.IRequest) {
	var data pb.SessionData
	if err := proto.Unmarshal(request.GetData(), &data); err != nil {
		zlog.Error(err)
		return
	}
	v, ok := sessions.Load(data.PlayerId)
	if !ok {
		return
	}
	s := v.(*Session)
	s.DisconnectedAt = 0
	s.GatewayID = data.GatewayId
	if len(data.ConnTags) > 0 {
		s.ConnTags = data.ConnTags
	}
	zlog.Ins().InfoF("SessionSvr: player %d reconnected (gateway=%s)", s.PlayerID, s.GatewayID)

	// Respond with restored session data
	resp, _ := proto.Marshal(&pb.SessionData{
		PlayerId:       s.PlayerID,
		GatewayId:      s.GatewayID,
		ConnTags:       s.ConnTags,
		DisconnectedAt: 0,
	})
	request.GetConnection().SendMsg(protocol.MsgIdSessionReconnect, resp)
}

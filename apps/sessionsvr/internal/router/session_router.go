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
	mu             sync.RWMutex
	PlayerID       int64
	GatewayID      string
	ConnTags       map[string]string
	DisconnectedAt int64 // unix timestamp, 0 = connected
}

var sessions sync.Map // playerId(int64) → *Session

type SessionRouter struct {
	znet.BaseRouter
	Reg    *pkg.Registry
	Server ziface.IServer // for broadcasting invalidation to all Gateways
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
	case protocol.MsgIdSessionInvalidate:
		// handled by Gateway, not SessionSvr — no-op
	}
}

func (r *SessionRouter) handleSave(request ziface.IRequest) {
	var data pb.SessionData
	if err := proto.Unmarshal(request.GetData(), &data); err != nil {
		zlog.Error(err)
		return
	}
	v, _ := sessions.LoadOrStore(data.PlayerId, &Session{PlayerID: data.PlayerId})
	s := v.(*Session)
	s.mu.Lock()
	s.GatewayID = data.GatewayId
	s.ConnTags = data.ConnTags
	s.DisconnectedAt = 0
	s.mu.Unlock()
}

func (r *SessionRouter) handleGet(request ziface.IRequest) {
	var data pb.SessionData
	if err := proto.Unmarshal(request.GetData(), &data); err != nil {
		zlog.Error(err)
		return
	}
	// Always respond — even for new players with no session — so the Gateway's
	// synchronous CheckReconnect doesn't time out on every first connection.
	resp := &pb.SessionData{PlayerId: data.PlayerId}
	if v, ok := sessions.Load(data.PlayerId); ok {
		s := v.(*Session)
		s.mu.RLock()
		resp.GatewayId = s.GatewayID
		resp.ConnTags = s.ConnTags
		resp.DisconnectedAt = s.DisconnectedAt
		s.mu.RUnlock()
	}
	respData, _ := proto.Marshal(resp)
	request.GetConnection().SendMsg(protocol.MsgIdSessionGet, respData)
}

func (r *SessionRouter) handleDisconnect(request ziface.IRequest) {
	var data pb.SessionData
	if err := proto.Unmarshal(request.GetData(), &data); err != nil {
		zlog.Error(err)
		return
	}
	v, _ := sessions.LoadOrStore(data.PlayerId, &Session{PlayerID: data.PlayerId})
	s := v.(*Session)
	s.mu.Lock()

	// If the player already reconnected to a different gateway, ignore this
	// disconnect from the stale gateway.
	if s.GatewayID != "" && s.GatewayID != data.GatewayId {
		s.mu.Unlock()
		zlog.Ins().InfoF("SessionSvr: ignoring stale disconnect player=%d from gw=%s (current: %s)",
			data.PlayerId, data.GatewayId, s.GatewayID)
		return
	}

	if s.GatewayID == "" {
		s.GatewayID = data.GatewayId
	}
	if s.ConnTags == nil {
		s.ConnTags = data.ConnTags
	}
	s.DisconnectedAt = time.Now().Unix()
	s.mu.Unlock()
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
	s.mu.Lock()
	oldGatewayID := s.GatewayID
	s.DisconnectedAt = 0
	s.GatewayID = data.GatewayId
	if len(data.ConnTags) > 0 {
		s.ConnTags = data.ConnTags
	}
	resp, _ := proto.Marshal(&pb.SessionData{
		PlayerId:       s.PlayerID,
		GatewayId:      s.GatewayID,
		ConnTags:       s.ConnTags,
		DisconnectedAt: 0,
	})
	s.mu.Unlock()
	zlog.Ins().InfoF("SessionSvr: player %d reconnected (gateway=%s)", s.PlayerID, s.GatewayID)
	request.GetConnection().SendMsg(protocol.MsgIdSessionReconnect, resp)

	// If the player switched gateways, broadcast an invalidation to all
	// connected gateways so the old one can clean up its stale PlayerConns entry.
	if oldGatewayID != "" && oldGatewayID != data.GatewayId && r.Server != nil {
		invData, _ := proto.Marshal(&pb.SessionData{
			PlayerId:  data.PlayerId,
			GatewayId: oldGatewayID,
		})
		zlog.Ins().InfoF("SessionSvr: broadcasting invalidation player=%d old=%s new=%s",
			data.PlayerId, oldGatewayID, data.GatewayId)
		r.Server.GetConnMgr().Range(func(_ uint64, conn ziface.IConnection, _ interface{}) error {
			conn.SendMsg(protocol.MsgIdSessionInvalidate, invData)
			return nil
		}, nil)
	}
}

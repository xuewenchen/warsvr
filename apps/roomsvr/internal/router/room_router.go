package router

import (
	"cardwar/pkg"
	"cardwar/pkg/conf"
	"cardwar/protocol"
	"cardwar/protocol/pb"
	"strconv"
	"sync"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/zlog"
	"github.com/aceld/zinx/znet"
	"google.golang.org/protobuf/proto"
)

var rooms sync.Map // matchId → []roomPlayer

type roomPlayer struct {
	playerID string
	conn     ziface.IConnection
	senderID uint64
}

type RoomRouter struct {
	znet.BaseRouter
	BC  pkg.Broadcaster
	Reg *pkg.Registry
}

func (r *RoomRouter) Handle(request ziface.IRequest) {
	var env pb.Envelope
	if err := proto.Unmarshal(request.GetData(), &env); err != nil {
		zlog.Error(err)
		return
	}

	msgID := request.GetMsgID()
	switch msgID {
	case protocol.MsgIdRoomJoinReq:
		r.handleJoin(&env, request.GetConnection())
	case protocol.MsgIdRoomLeaveReq:
		r.handleLeave(&env, request.GetConnection())
	case protocol.MsgIdSessionReconnected:
		r.handleReconnected(request)
	case protocol.MsgIdSessionForceLeave:
		r.handleForceLeave(request)
	}
}

func (r *RoomRouter) handleJoin(env *pb.Envelope, conn ziface.IConnection) {
	var req pb.RoomJoinReq
	if err := proto.Unmarshal(env.Data, &req); err != nil {
		r.sendJoinResp(conn, env.ConnId, req.MatchId, false, "invalid request")
		return
	}

	zlog.Ins().InfoF("handleJoin %s", req.String())

	playerID := env.ConnTags["player_id"]
	rp := roomPlayer{playerID: playerID, conn: conn, senderID: env.ConnId}

	raw, _ := rooms.LoadOrStore(req.MatchId, []roomPlayer{})
	players := raw.([]roomPlayer)
	for _, p := range players {
		if p.playerID == playerID {
			r.sendJoinResp(conn, env.ConnId, req.MatchId, false, "already in room")
			return
		}
	}
	players = append(players, rp)
	rooms.Store(req.MatchId, players)

	r.sendJoinResp(conn, env.ConnId, req.MatchId, true, "")

	// Broadcast to existing members
	if len(players) > 1 {
		r.broadcastRoomEvent(req.MatchId, players[:len(players)-1], playerID, "joined", len(players))
	}

	zlog.Ins().InfoF("RoomSvr: %s joined %s (%d players)", playerID, req.MatchId, len(players))
}

func (r *RoomRouter) handleLeave(env *pb.Envelope, conn ziface.IConnection) {
	var req pb.RoomLeaveReq
	if err := proto.Unmarshal(env.Data, &req); err != nil {
		return
	}

	zlog.Ins().InfoF("handleLeave %s", req.String())

	playerID := env.ConnTags["player_id"]

	v, ok := rooms.Load(req.MatchId)
	if !ok {
		return
	}
	players := v.([]roomPlayer)
	idx := -1
	for i, p := range players {
		if p.playerID == playerID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return
	}
	leftPlayer := players[idx]
	players = append(players[:idx], players[idx+1:]...)

	if len(players) == 0 {
		rooms.Delete(req.MatchId)
		r.notifyMatchSvr(req.MatchId)
		zlog.Ins().InfoF("RoomSvr: room %s destroyed (empty)", req.MatchId)
	} else {
		rooms.Store(req.MatchId, players)
		// Broadcast to remaining members
		r.broadcastRoomEvent(req.MatchId, players, playerID, "left", len(players))
		zlog.Ins().InfoF("RoomSvr: %s left %s (%d players)", playerID, req.MatchId, len(players))
	}

	resp, _ := proto.Marshal(&pb.RoomLeaveResp{Success: true})
	envResp, _ := proto.Marshal(&pb.Envelope{ConnId: leftPlayer.senderID, Data: resp})
	conn.SendMsg(protocol.MsgIdRoomLeaveResp, envResp)
}

func (r *RoomRouter) sendJoinResp(conn ziface.IConnection, senderID uint64, matchID string, success bool, errMsg string) {
	resp := &pb.RoomJoinResp{
		Success: success,
		MatchId: matchID,
		Error:   errMsg,
	}
	data, _ := proto.Marshal(resp)
	env, _ := proto.Marshal(&pb.Envelope{ConnId: senderID, Data: data})
	conn.SendMsg(protocol.MsgIdRoomJoinResp, env)
}

func (r *RoomRouter) notifyMatchSvr(matchID string) {
	if r.Reg == nil {
		return
	}
	conn := r.Reg.RouteTo(conf.SvcMatchSvr, matchID)
	if conn == nil {
		zlog.Ins().ErrorF("RoomSvr: no matchsvr connection for destroy notification")
		return
	}
	push, _ := proto.Marshal(&pb.RoomDestroyedPush{MatchId: matchID})
	env, _ := proto.Marshal(&pb.Envelope{Data: push})
	conn.SendMsg(protocol.MsgIdRoomDestroyedPush, env)
	zlog.Ins().InfoF("RoomSvr: notified MatchSvr of destroyed room %s", matchID)
}

func (r *RoomRouter) broadcastRoomEvent(matchId string, members []roomPlayer, playerID, event string, count int) {
	push, _ := proto.Marshal(&pb.RoomEventPush{
		MatchId:     matchId,
		PlayerId:    playerID,
		Event:       event,
		PlayerCount: int64(count),
	})
	for _, p := range members {
		r.BC.ToConn(protocol.MsgIdRoomEventPush, p.senderID, push, p.conn)
	}
}

// handleReconnected updates a room player's connection reference after reconnection.
// Called when Gateway notifies that a player has reconnected within TTL.
func (r *RoomRouter) handleReconnected(request ziface.IRequest) {
	var data pb.SessionData
	if err := proto.Unmarshal(request.GetData(), &data); err != nil {
		zlog.Error(err)
		return
	}
	matchID := data.ConnTags["match_id"]
	playerID := data.ConnTags["player_id"]
	senderID, _ := strconv.ParseUint(data.ConnTags["sender_id"], 10, 64)

	raw, ok := rooms.Load(matchID)
	if !ok {
		return
	}
	players := raw.([]roomPlayer)
	for i, p := range players {
		if p.playerID == playerID {
			players[i].conn = request.GetConnection()
			players[i].senderID = senderID
			rooms.Store(matchID, players)
			zlog.Ins().InfoF("RoomSvr: player %s reconnected in room %s", playerID, matchID)
			return
		}
	}
}

// handleForceLeave removes a player from a room when SessionSvr's TTL expires.
func (r *RoomRouter) handleForceLeave(request ziface.IRequest) {
	var data pb.SessionData
	if err := proto.Unmarshal(request.GetData(), &data); err != nil {
		zlog.Error(err)
		return
	}
	matchID := data.ConnTags["match_id"]
	playerID := strconv.FormatInt(data.PlayerId, 10)

	raw, ok := rooms.Load(matchID)
	if !ok {
		return
	}
	players := raw.([]roomPlayer)
	idx := -1
	for i, p := range players {
		if p.playerID == playerID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return
	}
	leftPlayer := players[idx]
	players = append(players[:idx], players[idx+1:]...)

	if len(players) == 0 {
		rooms.Delete(matchID)
		r.notifyMatchSvr(matchID)
		zlog.Ins().InfoF("RoomSvr: room %s destroyed (player %s force-left, TTL expired)", matchID, playerID)
	} else {
		rooms.Store(matchID, players)
		r.broadcastRoomEvent(matchID, players, playerID, "left", len(players))
	}

	// Send leave response to the leaving player via the original gateway conn
	resp, _ := proto.Marshal(&pb.RoomLeaveResp{Success: true})
	env, _ := proto.Marshal(&pb.Envelope{ConnId: leftPlayer.senderID, Data: resp})
	request.GetConnection().SendMsg(protocol.MsgIdRoomLeaveResp, env)
}

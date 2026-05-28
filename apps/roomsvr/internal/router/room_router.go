package router

import (
	"cardwar/pkg"
	"cardwar/protocol"
	"cardwar/protocol/pb"
	"sync"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/zlog"
	"github.com/aceld/zinx/znet"
	"google.golang.org/protobuf/proto"
)

var rooms sync.Map // matchId → []int64 (player IDs)

type RoomRouter struct {
	znet.BaseRouter
	BC  pkg.Broadcaster
	Srv ziface.IServer
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
	}
}

func (r *RoomRouter) handleJoin(env *pb.Envelope, conn ziface.IConnection) {
	var req pb.RoomJoinReq
	if err := proto.Unmarshal(env.Data, &req); err != nil {
		r.sendJoinResp(conn, env.ConnId, req.MatchId, false, "invalid request")
		return
	}

	senderPID := env.ConnTags["player_id"]

	// Auto-create room if first player
	actual, _ := rooms.LoadOrStore(req.MatchId, []string{})
	players := actual.([]string)
	for _, p := range players {
		if p == senderPID {
			r.sendJoinResp(conn, env.ConnId, req.MatchId, false, "already in room")
			return
		}
	}
	players = append(players, senderPID)
	rooms.Store(req.MatchId, players)

	r.sendJoinResp(conn, env.ConnId, req.MatchId, true, "")
	zlog.Ins().InfoF("RoomSvr: player %s joined room %s (%d players)", senderPID, req.MatchId, len(players))
}

func (r *RoomRouter) handleLeave(env *pb.Envelope, conn ziface.IConnection) {
	var req pb.RoomLeaveReq
	if err := proto.Unmarshal(env.Data, &req); err != nil {
		return
	}

	senderPID := env.ConnTags["player_id"]

	v, ok := rooms.Load(req.MatchId)
	if !ok {
		return
	}
	players := v.([]string)
	for i, p := range players {
		if p == senderPID {
			players = append(players[:i], players[i+1:]...)
			break
		}
	}
	if len(players) == 0 {
		rooms.Delete(req.MatchId)
		zlog.Ins().InfoF("RoomSvr: room %s destroyed (empty)", req.MatchId)
	} else {
		rooms.Store(req.MatchId, players)
		zlog.Ins().InfoF("RoomSvr: player %s left room %s (%d players)", senderPID, req.MatchId, len(players))
	}

	resp, _ := proto.Marshal(&pb.RoomLeaveResp{Success: true})
	envResp, _ := proto.Marshal(&pb.Envelope{ConnId: env.ConnId, Data: resp})
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

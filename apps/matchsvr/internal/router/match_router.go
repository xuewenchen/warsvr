package router

import (
	"cardwar/pkg"
	"cardwar/pkg/conf"
	"cardwar/protocol"
	"cardwar/protocol/pb"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/zlog"
	"github.com/aceld/zinx/znet"
	"google.golang.org/protobuf/proto"
)

var (
	activeMatches sync.Map // matchId → matchDir
	queues        sync.Map // matchType → []queuedPlayer
	loadCounts    sync.Map // serverId → int
)

type matchDir struct {
	ServerID  string
	MatchType string
	CreatedAt time.Time
}

type queuedPlayer struct {
	playerID int64
	elo      int64
	conn     ziface.IConnection
	senderID uint64
}

type MatchRouter struct {
	znet.BaseRouter
	BC  pkg.Broadcaster
	Srv ziface.IServer
}

func (r *MatchRouter) Handle(request ziface.IRequest) {
	var env pb.Envelope
	if err := proto.Unmarshal(request.GetData(), &env); err != nil {
		zlog.Error(err)
		return
	}

	msgID := request.GetMsgID()
	switch msgID {
	case protocol.MsgIdMatchEnterReq:
		r.handleEnter(&env, request.GetConnection())
	case protocol.MsgIdMatchAllocateReq:
		r.handleAllocate(&env, request.GetConnection())
	case protocol.MsgIdMatchQueryReq:
		r.handleQuery(&env, request.GetConnection())
	case protocol.MsgIdRoomDestroyedPush:
		r.handleDestroyed(&env)
	}
}

// ---- enter matchmaking pool ----

func (r *MatchRouter) handleEnter(env *pb.Envelope, conn ziface.IConnection) {
	var req pb.MatchEnterReq
	if err := proto.Unmarshal(env.Data, &req); err != nil {
		zlog.Error(err)
		return
	}

	senderPID, _ := strconv.ParseInt(env.ConnTags["player_id"], 10, 64)
	player := queuedPlayer{playerID: senderPID, elo: req.Elo, conn: conn, senderID: env.ConnId}

	raw, _ := queues.LoadOrStore(req.MatchType, []queuedPlayer{})
	pool := raw.([]queuedPlayer)
	pool = append(pool, player)
	queues.Store(req.MatchType, pool)

	needed := requiredPlayers(req.MatchType)
	if len(pool) >= needed {
		r.matchPool(req.MatchType, pool[:needed])
		queues.Store(req.MatchType, pool[needed:])
		return
	}

	resp, _ := proto.Marshal(&pb.MatchEnterResp{Status: "waiting", QueueSize: int64(len(pool))})
	envResp, _ := proto.Marshal(&pb.Envelope{ConnId: env.ConnId, Data: resp})
	conn.SendMsg(protocol.MsgIdMatchEnterResp, envResp)
	zlog.Ins().InfoF("MatchSvr: %s queue %d/%d", req.MatchType, len(pool), needed)
}

func (r *MatchRouter) matchPool(matchType string, pool []queuedPlayer) {
	serverID := r.pickLeastLoaded("roomsvr")
	matchID := fmt.Sprintf("match-%s-%d", matchType, time.Now().UnixNano())

	players := playerIDs(pool)
	activeMatches.Store(matchID, &matchDir{
		ServerID:  serverID,
		MatchType: matchType,
		CreatedAt: time.Now(),
	})
	incLoad(serverID)

	for _, p := range pool {
		push := &pb.MatchResultPush{
			MatchId:   matchID,
			ServerId:  serverID,
			Players:   players,
			MatchType: matchType,
		}
		data, _ := proto.Marshal(push)
		env := &pb.Envelope{
			ConnId: p.senderID,
			Data:   data,
			ConnTags: map[string]string{
				"server_id": serverID,
				"match_id":  matchID,
			},
		}
		envData, _ := proto.Marshal(env)
		p.conn.SendMsg(protocol.MsgIdMatchResultPush, envData)
	}
	zlog.Ins().InfoF("MatchSvr: matched %d players for %s on %s (%s)", len(pool), matchType, serverID, matchID)
}

// ---- allocate: assign roomsvr for a match ID ----

func (r *MatchRouter) handleAllocate(env *pb.Envelope, conn ziface.IConnection) {
	var req pb.MatchAllocateReq
	if err := proto.Unmarshal(env.Data, &req); err != nil {
		zlog.Error(err)
		return
	}

	if _, ok := activeMatches.Load(req.MatchId); ok {
		r.sendAllocateResp(conn, env.ConnId, req.MatchId, "", "already exists")
		return
	}

	serverID := r.pickLeastLoaded("roomsvr")
	if serverID == "" {
		r.sendAllocateResp(conn, env.ConnId, req.MatchId, "", "no roomsvr available")
		return
	}

	activeMatches.Store(req.MatchId, &matchDir{
		ServerID:  serverID,
		MatchType: "allocated",
		CreatedAt: time.Now(),
	})
	incLoad(serverID)

	r.sendAllocateResp(conn, env.ConnId, req.MatchId, serverID, "")
	zlog.Ins().InfoF("MatchSvr: allocated %s on %s", req.MatchId, serverID)
}

func (r *MatchRouter) sendAllocateResp(conn ziface.IConnection, senderID uint64, matchID, serverID, errMsg string) {
	resp, _ := proto.Marshal(&pb.MatchAllocateResp{MatchId: matchID, ServerId: serverID, Error: errMsg})
	tags := map[string]string{}
	if serverID != "" {
		tags["server_id"] = serverID
	}
	if matchID != "" {
		tags["match_id"] = matchID
	}
	env, _ := proto.Marshal(&pb.Envelope{ConnId: senderID, Data: resp, ConnTags: tags})
	conn.SendMsg(protocol.MsgIdMatchAllocateResp, env)
}

// ---- query: lookup match location ----

func (r *MatchRouter) handleQuery(env *pb.Envelope, conn ziface.IConnection) {
	var req pb.MatchQueryReq
	if err := proto.Unmarshal(env.Data, &req); err != nil {
		zlog.Error(err)
		return
	}

	v, ok := activeMatches.Load(req.MatchId)
	resp := &pb.MatchQueryResp{MatchId: req.MatchId}
	if ok {
		resp.ServerId = v.(*matchDir).ServerID
		resp.Found = true
	}
	tags := map[string]string{}
	if resp.Found {
		tags["server_id"] = resp.ServerId
		tags["match_id"] = req.MatchId
	}
	data, _ := proto.Marshal(resp)
	envResp, _ := proto.Marshal(&pb.Envelope{ConnId: env.ConnId, Data: data, ConnTags: tags})
	conn.SendMsg(protocol.MsgIdMatchQueryResp, envResp)
}

// ---- helpers ----

func (r *MatchRouter) pickLeastLoaded(service string) string {
	servers := conf.GlobalConfig.Services[service]
	if len(servers) == 0 {
		return ""
	}
	var best string
	minCount := int(^uint(0) >> 1)
	for _, srv := range servers {
		count := 0
		if v, ok := loadCounts.Load(srv.ID); ok {
			count = v.(int)
		}
		if count < minCount {
			minCount = count
			best = srv.ID
		}
	}
	return best
}

func incLoad(serverID string) {
	v, _ := loadCounts.LoadOrStore(serverID, 0)
	loadCounts.Store(serverID, v.(int)+1)
}

func playerIDs(pool []queuedPlayer) []int64 {
	ids := make([]int64, len(pool))
	for i, p := range pool {
		ids[i] = p.playerID
	}
	return ids
}

func requiredPlayers(matchType string) int {
	switch matchType {
	case "1v1":
		return 2
	case "2v2":
		return 4
	case "3v3":
		return 6
	case "5v5":
		return 10
	default:
		return 2
	}
}

func (r *MatchRouter) handleDestroyed(env *pb.Envelope) {
	var push pb.RoomDestroyedPush
	if err := proto.Unmarshal(env.Data, &push); err != nil {
		zlog.Error(err)
		return
	}
	v, ok := activeMatches.LoadAndDelete(push.MatchId)
	if !ok {
		return
	}
	dir := v.(*matchDir)
	decLoad(dir.ServerID)
	zlog.Ins().InfoF("MatchSvr: cleaned up match %s from %s", push.MatchId, dir.ServerID)
}

func decLoad(serverID string) {
	if v, ok := loadCounts.Load(serverID); ok {
		count := v.(int) - 1
		if count <= 0 {
			loadCounts.Delete(serverID)
		} else {
			loadCounts.Store(serverID, count)
		}
	}
}

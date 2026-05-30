package router

import (
	"strconv"

	"cardwar/pkg"
	"cardwar/pkg/conf"
	"cardwar/protocol"
	"cardwar/protocol/pb"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/zlog"
	"google.golang.org/protobuf/proto"
)

// CheckReconnect is called from OnConnStart after auth. It queries SessionSvr
// to see if this player has a previous session, and if so restores state.
func (gw *GatewayRef) CheckReconnect(playerID int64, conn ziface.IConnection) {
	if gw.Registry == nil {
		return
	}

	// Query SessionSvr
	req, _ := proto.Marshal(&pb.SessionData{PlayerId: playerID})
	sconn := gw.Registry.RouteTo(conf.SvcSessionSvr, strconv.FormatInt(playerID, 10))
	if sconn == nil {
		return
	}

	// Send SessionGet request — SessionSvr responds with SessionData if session exists
	sconn.SendMsg(protocol.MsgIdSessionGet, req)
}

// HandleSessionGet is called when SessionSvr responds with the session data.
func (gw *GatewayRef) HandleSessionGet(request ziface.IRequest) {
	var data pb.SessionData
	if err := proto.Unmarshal(request.GetData(), &data); err != nil {
		return
	}

	playerID := data.PlayerId
	val, ok := gw.PlayerConns.Load(playerID)
	if !ok {
		return
	}
	connID := val.(uint64)
	wsConn, err := gw.Server.GetConnMgr().Get(connID)
	if err != nil {
		return
	}

	if data.DisconnectedAt != 0 {
		// Player was disconnected — restore session
		for k, v := range data.ConnTags {
			wsConn.SetProperty(k, v)
		}
		zlog.Ins().InfoF("Gateway: player %d reconnected, restored session tags=%v", playerID, data.ConnTags)

		// Notify RoomSvr to update conn reference
		if matchID := data.ConnTags[pkg.TagMatchID]; matchID != "" {
			gw.notifyRoomReconnected(playerID, matchID, data.ConnTags[pkg.TagRoomSvrID], connID, wsConn)
		}

		// Tell SessionSvr the player reconnected
		reconnectData, _ := proto.Marshal(&pb.SessionData{
			PlayerId:  playerID,
			GatewayId: gw.ID,
		})
		if sconn := gw.Registry.RouteTo(conf.SvcSessionSvr, strconv.FormatInt(playerID, 10)); sconn != nil {
			sconn.SendMsg(protocol.MsgIdSessionReconnect, reconnectData)
		}
	} else {
		// Session exists but player is already "connected" (unexpected)
		// Just update conn_tags in session
		tags := gw.collectTags(wsConn)
		saveData, _ := proto.Marshal(&pb.SessionData{
			PlayerId:  playerID,
			GatewayId: gw.ID,
			ConnTags:  tags,
		})
		if sconn := gw.Registry.RouteTo(conf.SvcSessionSvr, strconv.FormatInt(playerID, 10)); sconn != nil {
			sconn.SendMsg(protocol.MsgIdSessionSave, saveData)
		}
	}
}

// MarkDisconnected tells SessionSvr the player disconnected (but keeps session alive for TTL).
func (gw *GatewayRef) MarkDisconnected(playerID int64) {
	if gw.Registry == nil {
		return
	}
	data, _ := proto.Marshal(&pb.SessionData{
		PlayerId:  playerID,
		GatewayId: gw.ID,
	})
	sconn := gw.Registry.RouteTo(conf.SvcSessionSvr, strconv.FormatInt(playerID, 10))
	if sconn != nil {
		sconn.SendMsg(protocol.MsgIdSessionDisconnect, data)
	}
}

// SyncSessionTags pushes the current connection tags to SessionSvr.
func (gw *GatewayRef) SyncSessionTags(conn ziface.IConnection) {
	if gw.Registry == nil {
		return
	}
	pidVal, err := conn.GetProperty(pkg.PropPlayerID)
	if err != nil {
		return
	}
	playerID := toPlayerID(pidVal)
	if playerID == 0 {
		return
	}
	tags := gw.collectTags(conn)
	if len(tags) == 0 {
		return
	}
	data, _ := proto.Marshal(&pb.SessionData{
		PlayerId:  playerID,
		GatewayId: gw.ID,
		ConnTags:  tags,
	})
	sconn := gw.Registry.RouteTo(conf.SvcSessionSvr, strconv.FormatInt(playerID, 10))
	if sconn != nil {
		sconn.SendMsg(protocol.MsgIdSessionSave, data)
	}
}

// 通知房间，玩家重新连接
func (gw *GatewayRef) notifyRoomReconnected(playerID int64, matchID, serverID string, connID uint64, conn ziface.IConnection) {
	key := serverID
	if key == "" {
		key = matchID
	}
	rconn := gw.Registry.RouteTo(conf.SvcRoomSvr, key)
	if rconn == nil {
		zlog.Ins().ErrorF("Gateway: no roomsvr connection to notify reconnect for player %d", playerID)
		return
	}
	data, _ := proto.Marshal(&pb.SessionData{
		PlayerId: playerID,
		ConnTags: map[string]string{
			pkg.TagPlayerID: strconv.FormatInt(playerID, 10),
			pkg.TagMatchID:  matchID,
			pkg.TagSenderID: strconv.FormatUint(connID, 10),
		},
	})
	rconn.SendMsg(protocol.MsgIdSessionReconnected, data)
}

func toPlayerID(v interface{}) int64 {
	switch id := v.(type) {
	case int64:
		return id
	case string:
		n, _ := strconv.ParseInt(id, 10, 64)
		return n
	}
	return 0
}

func (gw *GatewayRef) collectTags(conn ziface.IConnection) map[string]string {
	tags := make(map[string]string)
	for _, key := range pkg.SyncTagKeys {
		if v, err := conn.GetProperty(key); err == nil {
			if s, ok := v.(string); ok && s != "" {
				tags[key] = s
			}
		}
	}
	return tags
}

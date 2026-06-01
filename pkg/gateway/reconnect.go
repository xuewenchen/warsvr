package gateway

import (
	"strconv"
	"time"

	"cardwar/pkg/conf"
	"cardwar/pkg/connkey"
	"cardwar/protocol"
	"cardwar/protocol/pb"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/zlog"
	"google.golang.org/protobuf/proto"
)

const reconnectTimeout = 3 * time.Second

// CheckReconnect is called from OnConnStart after auth. It synchronously waits
// for SessionSvr to respond with session data, restoring conn_tags before the
// Zinx reader goroutine starts processing client messages. A timeout ensures
// OnConnStart doesn't hang if SessionSvr is unreachable.
func (gw *GatewayServer) CheckReconnect(playerID int64, conn ziface.IConnection) {
	if gw.Registry == nil {
		return
	}

	req, _ := proto.Marshal(&pb.SessionData{PlayerId: playerID})
	sconn := gw.Registry.RouteTo(conf.SvcSessionSvr, strconv.FormatInt(playerID, 10))
	if sconn == nil {
		return
	}

	ch := make(chan struct{})
	gw.pendingSessionGets.Store(playerID, ch)
	defer gw.pendingSessionGets.Delete(playerID)

	sconn.SendMsg(protocol.MsgIdSessionGet, req)

	select {
	case <-ch:
		zlog.Ins().InfoF("Gateway: CheckReconnect completed for player %d", playerID)
	case <-time.After(reconnectTimeout):
		zlog.Ins().ErrorF("Gateway: CheckReconnect timeout for player %d", playerID)
	}
}

// HandleSessionGet is called when SessionSvr responds with the session data.
func (gw *GatewayServer) HandleSessionGet(request ziface.IRequest) {
	var data pb.SessionData
	if err := proto.Unmarshal(request.GetData(), &data); err != nil {
		return
	}

	playerID := data.PlayerId

	// Signal the waiting CheckReconnect so OnConnStart can return and the
	// Zinx reader goroutine can start processing client messages. We must
	// signal on ALL exit paths — otherwise OnConnStart hangs until timeout.
	if ch, ok := gw.pendingSessionGets.LoadAndDelete(playerID); ok {
		defer close(ch.(chan struct{}))
	}

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
		if matchID := data.ConnTags[connkey.TagMatchID]; matchID != "" {
			gw.notifyRoomReconnected(playerID, matchID, data.ConnTags[connkey.TagRoomSvrID], connID, wsConn)
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
func (gw *GatewayServer) MarkDisconnected(playerID int64) {
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
func (gw *GatewayServer) SyncSessionTags(conn ziface.IConnection) {
	if gw.Registry == nil {
		return
	}
	pidVal, err := conn.GetProperty(connkey.PropPlayerID)
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
func (gw *GatewayServer) notifyRoomReconnected(playerID int64, matchID, serverID string, connID uint64, conn ziface.IConnection) {
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
			connkey.TagPlayerID: strconv.FormatInt(playerID, 10),
			connkey.TagMatchID:  matchID,
			connkey.TagSenderID: strconv.FormatUint(connID, 10),
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

func (gw *GatewayServer) collectTags(conn ziface.IConnection) map[string]string {
	tags := make(map[string]string)
	for _, key := range connkey.SyncTagKeys {
		if v, err := conn.GetProperty(key); err == nil {
			if s, ok := v.(string); ok && s != "" {
				tags[key] = s
			}
		}
	}
	return tags
}

// HandleSessionInvalidate is called when SessionSvr notifies this gateway that
// a player has reconnected to a different gateway. We remove the player from
// PlayerConns and stop the stale connection so Broadcaster.ToPlayer won't
// route messages to the dead connection.
func (gw *GatewayServer) HandleSessionInvalidate(request ziface.IRequest) {
	var data pb.SessionData
	if err := proto.Unmarshal(request.GetData(), &data); err != nil {
		return
	}
	// Only act if this gateway is the old gateway that needs invalidation.
	if data.GatewayId != gw.ID {
		return
	}
	playerID := data.PlayerId
	if playerID == 0 {
		return
	}

	val, ok := gw.PlayerConns.Load(playerID)
	if !ok {
		return // already cleaned up
	}
	connID := val.(uint64)
	gw.PlayerConns.Delete(playerID)

	wsConn, err := gw.Server.GetConnMgr().Get(connID)
	if err != nil {
		zlog.Ins().InfoF("Gateway: invalidate player %d conn not found (already gone)", playerID)
		return
	}

	zlog.Ins().InfoF("Gateway: invalidating player %d (reconnected elsewhere)", playerID)
	wsConn.Stop()
	// OnConnStop fires → PlayerConns.Delete (no-op, already deleted) →
	// MarkDisconnected → SessionSvr ignores (stale gateway check)
}

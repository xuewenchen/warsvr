package router

import (
	"time"

	"cardwar/pkg"
	"cardwar/pkg/conf"
	"cardwar/pkg/connkey"
	"cardwar/protocol"
	"cardwar/protocol/pb"

	"github.com/aceld/zinx/zlog"
	"google.golang.org/protobuf/proto"
)

// StartExpiryScanner runs a background goroutine that periodically scans
// for expired sessions and notifies backends to clean up.
func StartExpiryScanner(reg *pkg.Registry) {
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for range ticker.C {
			now := time.Now().Unix()
			sessions.Range(func(key, value interface{}) bool {
				s := value.(*Session)
				s.mu.RLock()
				if s.DisconnectedAt == 0 {
					s.mu.RUnlock()
					return true // still connected, skip
				}
				if now-s.DisconnectedAt < int64(SessionTTL.Seconds()) {
					s.mu.RUnlock()
					return true // not expired yet
				}
				s.mu.RUnlock()

				s.mu.Lock()
				// Re-check under write lock to avoid TOCTOU with handleReconnect/handleSave
				if s.DisconnectedAt != 0 && now-s.DisconnectedAt >= int64(SessionTTL.Seconds()) {
					zlog.Ins().InfoF("SessionSvr: session expired for player %d, cleaning up", s.PlayerID)

					matchID := s.ConnTags[connkey.TagMatchID]
					matchType := s.ConnTags[connkey.TagMatchType]
					roomSvrID := s.ConnTags[connkey.TagRoomSvrID]

					sessions.Delete(key)
					s.mu.Unlock()

					if matchID != "" {
						notifyForceLeaveRoom(reg, s.PlayerID, matchID, roomSvrID)
					}
					if matchType != "" {
						notifyForceLeaveQueue(reg, s.PlayerID, matchType)
					}
				} else {
					s.mu.Unlock()
				}

				return true
			})
		}
	}()
}

func notifyForceLeaveRoom(reg *pkg.Registry, playerID int64, matchID, serverID string) {
	key := serverID
	if key == "" {
		key = matchID
	}
	conn := reg.RouteTo(conf.SvcRoomSvr, key)
	if conn == nil {
		zlog.Ins().ErrorF("SessionSvr: no roomsvr connection for force leave (player=%d, match=%s)", playerID, matchID)
		return
	}
	data, _ := proto.Marshal(&pb.SessionData{
		PlayerId: playerID,
		ConnTags: map[string]string{connkey.TagMatchID: matchID},
	})
	conn.SendMsg(protocol.MsgIdSessionForceLeave, data)
	zlog.Ins().InfoF("SessionSvr: force leave room player=%d match=%s", playerID, matchID)
}

func notifyForceLeaveQueue(reg *pkg.Registry, playerID int64, matchType string) {
	conn := reg.RouteTo(conf.SvcMatchSvr, matchType)
	if conn == nil {
		zlog.Ins().ErrorF("SessionSvr: no matchsvr connection for force leave queue (player=%d)", playerID)
		return
	}
	data, _ := proto.Marshal(&pb.SessionData{
		PlayerId: playerID,
		ConnTags: map[string]string{connkey.TagMatchType: matchType},
	})
	conn.SendMsg(protocol.MsgIdSessionForceLeaveQueue, data)
	zlog.Ins().InfoF("SessionSvr: force leave queue player=%d type=%s", playerID, matchType)
}

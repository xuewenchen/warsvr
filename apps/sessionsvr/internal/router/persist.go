package router

import (
	"encoding/json"
	"time"

	"cardwar/pkg/persist"

	"github.com/aceld/zinx/zlog"
)

var store *persist.Store

// sessionPersist is the serializable form of a session (no mutex).
type sessionPersist struct {
	PlayerID       int64             `json:"player_id"`
	GatewayID      string            `json:"gateway_id"`
	ConnTags       map[string]string `json:"conn_tags,omitempty"`
	DisconnectedAt int64             `json:"disconnected_at"`
}

type journalEntry struct {
	OP             string            `json:"op"`
	PlayerID       int64             `json:"player_id"`
	GatewayID      string            `json:"gateway_id,omitempty"`
	ConnTags       map[string]string `json:"conn_tags,omitempty"`
	DisconnectedAt int64             `json:"disconnected_at,omitempty"`
	TS             int64             `json:"ts"`
}

// InitStore creates the persist.Store and recovers session state.
// dir is the data directory, e.g. "data/sessionsvr".
func InitStore(dir string) error {
	var err error
	store, err = persist.OpenStore(persist.StoreConfig{
		Dir:          dir,
		Snapshot:     snapshotSessions,
		LoadSnapshot: loadSnapshot,
		Replay:       replaySession,
	})
	return err
}

// StartAutoSnapshot starts periodic snapshotting.
func StartAutoSnapshot() {
	if store != nil {
		store.StartAutoSnapshot()
	}
}

// CloseStore flushes and closes the journal.
func CloseStore() {
	if store != nil {
		store.Close()
	}
}

func snapshotSessions() ([]byte, error) {
	state := make(map[int64]*sessionPersist)
	sessions.Range(func(key, value interface{}) bool {
		s := value.(*Session)
		s.mu.RLock()
		state[s.PlayerID] = &sessionPersist{
			PlayerID:       s.PlayerID,
			GatewayID:      s.GatewayID,
			ConnTags:       copyTags(s.ConnTags),
			DisconnectedAt: s.DisconnectedAt,
		}
		s.mu.RUnlock()
		return true
	})
	return json.Marshal(state)
}

func loadSnapshot(data []byte) error {
	var state map[int64]*sessionPersist
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}
	for _, sp := range state {
		s := &Session{
			PlayerID:       sp.PlayerID,
			GatewayID:      sp.GatewayID,
			ConnTags:       sp.ConnTags,
			DisconnectedAt: sp.DisconnectedAt,
		}
		sessions.Store(sp.PlayerID, s)
	}
	return nil
}

func replaySession(entry []byte) error {
	var e journalEntry
	if err := json.Unmarshal(entry, &e); err != nil {
		return err
	}

	switch e.OP {
	case "save":
		v, _ := sessions.LoadOrStore(e.PlayerID, &Session{PlayerID: e.PlayerID})
		s := v.(*Session)
		s.mu.Lock()
		s.GatewayID = e.GatewayID
		if e.ConnTags != nil {
			s.ConnTags = e.ConnTags
		}
		s.DisconnectedAt = 0
		s.mu.Unlock()

	case "disconnect":
		v, ok := sessions.Load(e.PlayerID)
		if !ok {
			return nil
		}
		s := v.(*Session)
		s.mu.Lock()
		if s.GatewayID != "" && s.GatewayID != e.GatewayID {
			s.mu.Unlock()
			return nil // stale disconnect
		}
		if s.GatewayID == "" {
			s.GatewayID = e.GatewayID
		}
		if s.ConnTags == nil {
			s.ConnTags = e.ConnTags
		}
		if e.DisconnectedAt != 0 {
			s.DisconnectedAt = e.DisconnectedAt
		}
		s.mu.Unlock()

	case "reconnect":
		v, ok := sessions.Load(e.PlayerID)
		if !ok {
			return nil
		}
		s := v.(*Session)
		s.mu.Lock()
		s.DisconnectedAt = 0
		s.GatewayID = e.GatewayID
		if e.ConnTags != nil {
			s.ConnTags = e.ConnTags
		}
		s.mu.Unlock()
	}
	return nil
}

func copyTags(src map[string]string) map[string]string {
	if src == nil {
		return nil
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func journalAppend(e journalEntry) {
	if store == nil {
		return
	}
	e.TS = time.Now().UnixMilli()
	data, err := json.Marshal(e)
	if err != nil {
		zlog.Ins().ErrorF("SessionSvr: journal marshal failed: %v", err)
		return
	}
	if err := store.Append(data); err != nil {
		zlog.Ins().ErrorF("SessionSvr: journal append failed: %v", err)
	}
}

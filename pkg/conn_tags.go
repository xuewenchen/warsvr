package pkg

// Well-known connection property keys — used with conn.SetProperty / conn.GetProperty.
const (
	PropPlayerID = "playerId"  // client player identity (int64)
	PropServerID = "server_id" // backend instance identifier
	PropConnType = "conn_type" // service identity string, e.g. "gateway" / "roomsvr"
)

// Well-known Envelope.ConnTags / SessionData.ConnTags keys.
const (
	TagPlayerID       = "player_id"        // sender player identity
	TagTargetPlayerID = "target_player_id" // private message target player
	TagRoomSvrID      = "room_server_id"   // roomsvr instance assigned by MatchSvr
	TagMatchID        = "match_id"         // match/room identifier
	TagMatchType      = "match_type"       // match type: "1v1", "2v2", etc.
	TagSenderID       = "sender_id"        // client connID (used during reconnect)
)

// SyncTagKeys lists the conn_tags keys that Gateway syncs to SessionSvr
// whenever a backend response sets them on a client connection.
var SyncTagKeys = []string{TagRoomSvrID, TagMatchID, TagMatchType}

package common

import "encoding/json"

const (
	MsgIdPing      = 1
	MsgIdPong      = 2
	MsgIdLogin     = 3
	MsgIdChat      = 4
	MsgIdLoginRsp  = 5
	MsgIdBroadcast = 6
)

// External protocol: Client <-> Gateway

type LoginMsg struct {
	PlayerID string `json:"player_id"`
}

type LoginRspMsg struct {
	PlayerID string `json:"player_id"`
	Success  bool   `json:"success"`
	Message  string `json:"message,omitempty"`
}

type ChatMsg struct {
	PlayerID string `json:"player_id"`
	Content  string `json:"content"`
}

type BroadcastMsg struct {
	PlayerID  string `json:"player_id"`
	Content   string `json:"content"`
	Timestamp int64  `json:"timestamp"`
}

// Internal protocol: Gateway <-> ChatSvr
// ConnID=0 means broadcast to all clients on the gateway.

type Envelope struct {
	ConnID uint64          `json:"conn_id"`
	Data   json.RawMessage `json:"data"`
}

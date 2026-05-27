package protocol

// Message IDs for Zinx routing.
const (
	// ping pong
	MsgIdPing = 1
	MsgIdPong = 2

	// ===== chat
	MsgIdChat = 4

	// ===== chat push from backend to clients
	MsgIdChatPush = 6
)

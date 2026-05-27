package protocol

// Message IDs for Zinx routing.
const (
	// ping pong
	MsgIdPing = 1
	MsgIdPong = 2

	// ===== chat
	MsgIdChatReq  = 5
	MsgIdChatResp = 6

	// ===== internal
	MsgIdGatewayRegister = 100 // Gateway identifies itself to backends on connect
)

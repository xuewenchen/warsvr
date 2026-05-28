package protocol

import "cardwar/protocol/pb"

// Message IDs for Zinx routing. Defined in protocol/proto/msgid.proto.
const (
	MsgIdPing     = uint32(pb.MsgID_PING)
	MsgIdPong     = uint32(pb.MsgID_PONG)
	MsgIdChatReq  = uint32(pb.MsgID_CHAT_REQ)
	MsgIdChatResp = uint32(pb.MsgID_CHAT_RESP)
)

// gateway注册后端服务时使用的消息ID
const (
	MsgIdGatewayRegister = uint32(pb.MsgID_GATEWAY_REGISTER)
)

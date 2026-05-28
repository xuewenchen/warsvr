package protocol

import "cardwar/protocol/pb"

// Message IDs for Zinx routing. Defined in protocol/proto/msgid.proto.
const (
	MsgIdPing     = uint32(pb.MsgID_PING)
	MsgIdPong     = uint32(pb.MsgID_PONG)
	MsgIdChatReq  = uint32(pb.MsgID_CHAT_REQ)
	MsgIdChatResp = uint32(pb.MsgID_CHAT_RESP)

	// ===== match pool
	MsgIdMatchEnterReq   = uint32(pb.MsgID_MATCH_ENTER_REQ)
	MsgIdMatchEnterResp  = uint32(pb.MsgID_MATCH_ENTER_RESP)
	MsgIdMatchResultPush  = uint32(pb.MsgID_MATCH_RESULT_PUSH)

	// ===== match directory
	MsgIdMatchAllocateReq  = uint32(pb.MsgID_MATCH_ALLOCATE_REQ)
	MsgIdMatchAllocateResp = uint32(pb.MsgID_MATCH_ALLOCATE_RESP)
	MsgIdMatchQueryReq     = uint32(pb.MsgID_MATCH_QUERY_REQ)
	MsgIdMatchQueryResp    = uint32(pb.MsgID_MATCH_QUERY_RESP)

	// ===== room
	MsgIdRoomJoinReq   = uint32(pb.MsgID_ROOM_JOIN_REQ)
	MsgIdRoomJoinResp  = uint32(pb.MsgID_ROOM_JOIN_RESP)
	MsgIdRoomLeaveReq  = uint32(pb.MsgID_ROOM_LEAVE_REQ)
	MsgIdRoomLeaveResp = uint32(pb.MsgID_ROOM_LEAVE_RESP)

	MsgIdRoomDestroyedPush = uint32(pb.MsgID_ROOM_DESTROYED_PUSH)
	MsgIdRoomEventPush    = uint32(pb.MsgID_ROOM_EVENT_PUSH)
)

const (
	MsgIdGatewayRegister = uint32(pb.MsgID_GATEWAY_REGISTER)
)

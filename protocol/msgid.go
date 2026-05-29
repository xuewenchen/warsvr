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
	MsgIdMatchResultPush = uint32(pb.MsgID_MATCH_RESULT_PUSH)

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
	MsgIdRoomEventPush     = uint32(pb.MsgID_ROOM_EVENT_PUSH)

	// ===== session
	MsgIdSessionSave            = uint32(pb.MsgID_SESSION_SAVE)
	MsgIdSessionGet             = uint32(pb.MsgID_SESSION_GET)
	MsgIdSessionDisconnect      = uint32(pb.MsgID_SESSION_DISCONNECT)
	MsgIdSessionReconnect       = uint32(pb.MsgID_SESSION_RECONNECT)
	MsgIdSessionForceLeave      = uint32(pb.MsgID_SESSION_FORCE_LEAVE)
	MsgIdSessionForceLeaveQueue = uint32(pb.MsgID_SESSION_FORCE_LEAVE_QUEUE)
	MsgIdSessionReconnected     = uint32(pb.MsgID_SESSION_RECONNECTED)
)

const (
	MsgIdServiceIdentity = uint32(pb.MsgID_SERVICE_IDENTITY)
)

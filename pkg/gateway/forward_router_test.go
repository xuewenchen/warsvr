package gateway

import (
	"cardwar/protocol"
	"cardwar/protocol/pb"
	"testing"

	"google.golang.org/protobuf/proto"
)

// ── sendError tests: verify correct response proto type per msgID ──────────

func TestSendError_ChatReq(t *testing.T) {
	conn := &mockConn{}
	r := &ForwardRouter{GW: &GatewayServer{}}

	r.sendError(conn, protocol.MsgIdChatReq)

	conn.mu.Lock()
	msgs := conn.sent
	conn.mu.Unlock()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(msgs))
	}
	if msgs[0].msgID != protocol.MsgIdChatResp {
		t.Errorf("msgID: got %d, want %d", msgs[0].msgID, protocol.MsgIdChatResp)
	}

	var resp pb.ChatResp
	if err := proto.Unmarshal(msgs[0].data, &resp); err != nil {
		t.Fatalf("unmarshal ChatResp: %v", err)
	}
	if resp.SenderPlayerId != -1 {
		t.Errorf("SenderPlayerId: got %d, want -1", resp.SenderPlayerId)
	}
	if resp.Content == "" {
		t.Error("Content should not be empty")
	}
}

func TestSendError_MatchEnterReq(t *testing.T) {
	conn := &mockConn{}
	r := &ForwardRouter{GW: &GatewayServer{}}

	r.sendError(conn, protocol.MsgIdMatchEnterReq)

	conn.mu.Lock()
	msgs := conn.sent
	conn.mu.Unlock()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(msgs))
	}
	if msgs[0].msgID != protocol.MsgIdMatchEnterResp {
		t.Errorf("msgID: got %d, want %d", msgs[0].msgID, protocol.MsgIdMatchEnterResp)
	}

	var resp pb.MatchEnterResp
	if err := proto.Unmarshal(msgs[0].data, &resp); err != nil {
		t.Fatalf("unmarshal MatchEnterResp: %v", err)
	}
	if resp.Status != "error" {
		t.Errorf("Status: got %s, want error", resp.Status)
	}
}

func TestSendError_MatchAllocateReq(t *testing.T) {
	conn := &mockConn{}
	r := &ForwardRouter{GW: &GatewayServer{}}

	r.sendError(conn, protocol.MsgIdMatchAllocateReq)

	conn.mu.Lock()
	msgs := conn.sent
	conn.mu.Unlock()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(msgs))
	}
	if msgs[0].msgID != protocol.MsgIdMatchAllocateResp {
		t.Errorf("msgID: got %d, want %d", msgs[0].msgID, protocol.MsgIdMatchAllocateResp)
	}

	var resp pb.MatchAllocateResp
	if err := proto.Unmarshal(msgs[0].data, &resp); err != nil {
		t.Fatalf("unmarshal MatchAllocateResp: %v", err)
	}
	if resp.Error == "" {
		t.Error("Error should not be empty")
	}
}

func TestSendError_MatchQueryReq(t *testing.T) {
	conn := &mockConn{}
	r := &ForwardRouter{GW: &GatewayServer{}}

	r.sendError(conn, protocol.MsgIdMatchQueryReq)

	conn.mu.Lock()
	msgs := conn.sent
	conn.mu.Unlock()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(msgs))
	}
	if msgs[0].msgID != protocol.MsgIdMatchQueryResp {
		t.Errorf("msgID: got %d, want %d", msgs[0].msgID, protocol.MsgIdMatchQueryResp)
	}

	var resp pb.MatchQueryResp
	if err := proto.Unmarshal(msgs[0].data, &resp); err != nil {
		t.Fatalf("unmarshal MatchQueryResp: %v", err)
	}
	if resp.Found {
		t.Error("Found should be false")
	}
}

func TestSendError_RoomJoinReq(t *testing.T) {
	conn := &mockConn{}
	r := &ForwardRouter{GW: &GatewayServer{}}

	r.sendError(conn, protocol.MsgIdRoomJoinReq)

	conn.mu.Lock()
	msgs := conn.sent
	conn.mu.Unlock()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(msgs))
	}
	if msgs[0].msgID != protocol.MsgIdRoomJoinResp {
		t.Errorf("msgID: got %d, want %d", msgs[0].msgID, protocol.MsgIdRoomJoinResp)
	}

	var resp pb.RoomJoinResp
	if err := proto.Unmarshal(msgs[0].data, &resp); err != nil {
		t.Fatalf("unmarshal RoomJoinResp: %v", err)
	}
	if resp.Success {
		t.Error("Success should be false")
	}
	if resp.Error == "" {
		t.Error("Error should not be empty")
	}
}

func TestSendError_RoomLeaveReq(t *testing.T) {
	conn := &mockConn{}
	r := &ForwardRouter{GW: &GatewayServer{}}

	r.sendError(conn, protocol.MsgIdRoomLeaveReq)

	conn.mu.Lock()
	msgs := conn.sent
	conn.mu.Unlock()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(msgs))
	}
	if msgs[0].msgID != protocol.MsgIdRoomLeaveResp {
		t.Errorf("msgID: got %d, want %d", msgs[0].msgID, protocol.MsgIdRoomLeaveResp)
	}

	var resp pb.RoomLeaveResp
	if err := proto.Unmarshal(msgs[0].data, &resp); err != nil {
		t.Fatalf("unmarshal RoomLeaveResp: %v", err)
	}
	if resp.Success {
		t.Error("Success should be false")
	}
	if resp.Error == "" {
		t.Error("Error should not be empty")
	}
}

func TestSendError_UnknownMsgID(t *testing.T) {
	conn := &mockConn{}
	r := &ForwardRouter{GW: &GatewayServer{}}

	// Unknown msgID 255 → should send msgID 256 with empty body
	r.sendError(conn, 255)

	conn.mu.Lock()
	msgs := conn.sent
	conn.mu.Unlock()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(msgs))
	}
	if msgs[0].msgID != 256 {
		t.Errorf("msgID: got %d, want 256", msgs[0].msgID)
	}
	if len(msgs[0].data) != 0 {
		t.Errorf("data for unknown msgID should be nil, got %d bytes", len(msgs[0].data))
	}
}

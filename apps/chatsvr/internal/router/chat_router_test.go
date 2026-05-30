package router

import (
	"cardwar/pkg"
	"cardwar/protocol/pb"
	"strconv"
	"sync"
	"testing"

	"github.com/aceld/zinx/ziface"
	"google.golang.org/protobuf/proto"
)

// mockBC implements pkg.Broadcaster, records calls for assertions.
type mockBC struct {
	mu            sync.Mutex
	toAllCalls    int
	toPlayerCalls []toPlayerCall
	toConnCalls   []toConnCall
}

type toPlayerCall struct {
	msgID     uint32
	targetPID int64
}
type toConnCall struct {
	msgID  uint32
	connID uint64
}

func (m *mockBC) ToAll(msgID uint32, _ []byte) {
	m.mu.Lock()
	m.toAllCalls++
	m.mu.Unlock()
}
func (m *mockBC) ToPlayer(msgID uint32, targetPID int64, _ []byte) {
	m.mu.Lock()
	m.toPlayerCalls = append(m.toPlayerCalls, toPlayerCall{msgID, targetPID})
	m.mu.Unlock()
}
func (m *mockBC) ToConn(msgID uint32, connID uint64, _ []byte, _ ziface.IConnection) {
	m.mu.Lock()
	m.toConnCalls = append(m.toConnCalls, toConnCall{msgID, connID})
	m.mu.Unlock()
}

type testConn struct {
	ziface.IConnection
	id uint64
}

func (c *testConn) GetConnID() uint64 { return c.id }

type testRequest struct {
	ziface.IRequest
	msgID uint32
	data  []byte
	conn  ziface.IConnection
}

func (r *testRequest) GetMsgID() uint32                  { return r.msgID }
func (r *testRequest) GetData() []byte                   { return r.data }
func (r *testRequest) GetConnection() ziface.IConnection { return r.conn }

func makeChatEnvelope(senderPID int64, content string, targetPID int64) []byte {
	chatReq, _ := proto.Marshal(&pb.ChatReq{Content: content, TargetPlayerId: targetPID})
	env, _ := proto.Marshal(&pb.Envelope{
		ConnId:   99,
		Data:     chatReq,
		ConnTags: map[string]string{pkg.TagPlayerID: i64s(senderPID)},
	})
	return env
}

func i64s(v int64) string { return strconv.FormatInt(v, 10) }

func TestChatRouter_GlobalChat(t *testing.T) {
	mbc := &mockBC{}
	r := &ChatRouter{BC: mbc}

	env := makeChatEnvelope(1, "hello world", 0)
	req := &testRequest{msgID: 1, data: env, conn: &testConn{id: 99}}
	r.Handle(req)

	if mbc.toAllCalls != 1 {
		t.Errorf("global chat: expected 1 ToAll call, got %d", mbc.toAllCalls)
	}
	if len(mbc.toPlayerCalls) != 0 {
		t.Errorf("global chat: expected 0 ToPlayer calls, got %d", len(mbc.toPlayerCalls))
	}
}

func TestChatRouter_PrivateChat(t *testing.T) {
	mbc := &mockBC{}
	r := &ChatRouter{BC: mbc}

	env := makeChatEnvelope(1, "secret msg", 2)
	req := &testRequest{msgID: 1, data: env, conn: &testConn{id: 99}}
	r.Handle(req)

	if len(mbc.toPlayerCalls) != 1 {
		t.Fatalf("private chat: expected 1 ToPlayer call, got %d", len(mbc.toPlayerCalls))
	}
	if mbc.toPlayerCalls[0].targetPID != 2 {
		t.Errorf("private chat: expected targetPID 2, got %d", mbc.toPlayerCalls[0].targetPID)
	}
	if len(mbc.toConnCalls) != 1 {
		t.Fatalf("private chat: expected 1 ToConn call, got %d", len(mbc.toConnCalls))
	}
}

func TestChatRouter_InvalidEnvelope(t *testing.T) {
	mbc := &mockBC{}
	r := &ChatRouter{BC: mbc}
	req := &testRequest{msgID: 1, data: []byte("garbage"), conn: &testConn{id: 1}}
	// Should not panic; just log error and return
	r.Handle(req)
	if mbc.toAllCalls != 0 {
		t.Error("expected no calls with invalid data")
	}
}

func TestChatRouter_EmptyContent(t *testing.T) {
	mbc := &mockBC{}
	r := &ChatRouter{BC: mbc}

	env := makeChatEnvelope(1, "", 0)
	req := &testRequest{msgID: 1, data: env, conn: &testConn{id: 1}}
	r.Handle(req)

	if mbc.toAllCalls != 1 {
		t.Error("empty content should still broadcast")
	}
}

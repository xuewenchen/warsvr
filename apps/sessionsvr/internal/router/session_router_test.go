package router

import (
	"cardwar/pkg"
	"cardwar/protocol"
	"cardwar/protocol/pb"
	"context"
	"errors"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
)

// ── mock types (session-specific, lighter than gateway mocks) ───────────────

type sessionMockRequest struct {
	msgID uint32
	data  []byte
	conn  ziface.IConnection
}

func (m *sessionMockRequest) GetConnection() ziface.IConnection       { return m.conn }
func (m *sessionMockRequest) GetData() []byte                         { return m.data }
func (m *sessionMockRequest) GetMsgID() uint32                        { return m.msgID }
func (m *sessionMockRequest) GetMessage() ziface.IMessage             { return nil }
func (m *sessionMockRequest) GetResponse() ziface.IcResp              { return nil }
func (m *sessionMockRequest) SetResponse(ziface.IcResp)               {}
func (m *sessionMockRequest) BindRouter(ziface.IRouter)               {}
func (m *sessionMockRequest) Call()                                   {}
func (m *sessionMockRequest) Abort()                                  {}
func (m *sessionMockRequest) Goto(ziface.HandleStep)                  {}
func (m *sessionMockRequest) BindRouterSlices([]ziface.RouterHandler) {}
func (m *sessionMockRequest) RouterSlicesNext()                       {}
func (m *sessionMockRequest) Copy() ziface.IRequest                   { return m }
func (m *sessionMockRequest) Set(string, interface{})                 {}
func (m *sessionMockRequest) Get(string) (interface{}, bool)          { return nil, false }

type sessionMockConn struct {
	mu        sync.Mutex
	sent      []sentMsg // captured SendMsg calls
	stopCount int
	props     map[string]interface{}
}

type sentMsg struct {
	msgID uint32
	data  []byte
}

func (m *sessionMockConn) Start()                                            {}
func (m *sessionMockConn) Stop()                                             { m.mu.Lock(); m.stopCount++; m.mu.Unlock() }
func (m *sessionMockConn) Context() context.Context                          { return nil }
func (m *sessionMockConn) GetName() string                                   { return "" }
func (m *sessionMockConn) GetConnection() net.Conn                           { return nil }
func (m *sessionMockConn) GetWsConn() *websocket.Conn                        { return nil }
func (m *sessionMockConn) GetTCPConnection() net.Conn                        { return nil }
func (m *sessionMockConn) GetConnID() uint64                                 { return 0 }
func (m *sessionMockConn) GetConnIdStr() string                              { return "" }
func (m *sessionMockConn) GetMsgHandler() ziface.IMsgHandle                  { return nil }
func (m *sessionMockConn) GetWorkerID() uint32                               { return 0 }
func (m *sessionMockConn) RemoteAddr() net.Addr                              { return nil }
func (m *sessionMockConn) LocalAddr() net.Addr                               { return nil }
func (m *sessionMockConn) LocalAddrString() string                           { return "" }
func (m *sessionMockConn) RemoteAddrString() string                          { return "" }
func (m *sessionMockConn) Send([]byte) error                                 { return nil }
func (m *sessionMockConn) SendToQueue([]byte, ...ziface.MsgSendOption) error { return nil }
func (m *sessionMockConn) SendMsg(msgID uint32, data []byte) error {
	m.mu.Lock()
	m.sent = append(m.sent, sentMsg{msgID: msgID, data: data})
	m.mu.Unlock()
	return nil
}
func (m *sessionMockConn) SendBuffMsg(uint32, []byte, ...ziface.MsgSendOption) error { return nil }
func (m *sessionMockConn) SetProperty(k string, v interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.props == nil {
		m.props = make(map[string]interface{})
	}
	m.props[k] = v
}
func (m *sessionMockConn) GetProperty(k string) (interface{}, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if v, ok := m.props[k]; ok {
		return v, nil
	}
	return nil, nil
}
func (m *sessionMockConn) RemoveProperty(string)                             {}
func (m *sessionMockConn) IsAlive() bool                                     { return true }
func (m *sessionMockConn) SetHeartBeat(ziface.IHeartbeatChecker)             {}
func (m *sessionMockConn) AddCloseCallback(interface{}, interface{}, func()) {}
func (m *sessionMockConn) RemoveCloseCallback(interface{}, interface{})      {}
func (m *sessionMockConn) InvokeCloseCallbacks()                             {}
func (m *sessionMockConn) lastMsg() (uint32, []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.sent) == 0 {
		return 0, nil
	}
	last := m.sent[len(m.sent)-1]
	return last.msgID, last.data
}

// sessionMockServer: lightweight IServer stub for testing invalidation broadcast.
type sessionMockServer struct {
	connMgr ziface.IConnManager
}

func (m *sessionMockServer) Start()                           {}
func (m *sessionMockServer) Stop()                            {}
func (m *sessionMockServer) Serve()                           {}
func (m *sessionMockServer) AddRouter(uint32, ziface.IRouter) {}
func (m *sessionMockServer) AddRouterSlices(uint32, ...ziface.RouterHandler) ziface.IRouterSlices {
	return nil
}
func (m *sessionMockServer) Group(uint32, uint32, ...ziface.RouterHandler) ziface.IGroupRouterSlices {
	return nil
}
func (m *sessionMockServer) Use(...ziface.RouterHandler) ziface.IRouterSlices                { return nil }
func (m *sessionMockServer) GetConnMgr() ziface.IConnManager                                 { return m.connMgr }
func (m *sessionMockServer) SetOnConnStart(func(ziface.IConnection))                         {}
func (m *sessionMockServer) SetOnConnStop(func(ziface.IConnection))                          {}
func (m *sessionMockServer) GetOnConnStart() func(ziface.IConnection)                        { return nil }
func (m *sessionMockServer) GetOnConnStop() func(ziface.IConnection)                         { return nil }
func (m *sessionMockServer) GetPacket() ziface.IDataPack                                     { return nil }
func (m *sessionMockServer) GetMsgHandler() ziface.IMsgHandle                                { return nil }
func (m *sessionMockServer) SetPacket(ziface.IDataPack)                                      {}
func (m *sessionMockServer) StartHeartBeat(time.Duration)                                    {}
func (m *sessionMockServer) StartHeartBeatWithOption(time.Duration, *ziface.HeartBeatOption) {}
func (m *sessionMockServer) GetHeartBeat() ziface.IHeartbeatChecker                          { return nil }
func (m *sessionMockServer) GetLengthField() *ziface.LengthField                             { return nil }
func (m *sessionMockServer) SetDecoder(ziface.IDecoder)                                      {}
func (m *sessionMockServer) AddInterceptor(ziface.IInterceptor)                              {}
func (m *sessionMockServer) SetWebsocketAuth(func(*http.Request) error)                      {}
func (m *sessionMockServer) ServerName() string                                              { return "mock" }

// sessionMockConnMgr: captures Range invocations for invalidation broadcast tests.
type sessionMockConnMgr struct {
	conns []ziface.IConnection
}

func (m *sessionMockConnMgr) Add(ziface.IConnection)    { panic("unexpected") }
func (m *sessionMockConnMgr) Remove(ziface.IConnection) { panic("unexpected") }
func (m *sessionMockConnMgr) Get(uint64) (ziface.IConnection, error) {
	return nil, errors.New("unexpected")
}
func (m *sessionMockConnMgr) Get2(string) (ziface.IConnection, error) {
	return nil, errors.New("unexpected")
}
func (m *sessionMockConnMgr) Len() int                  { return 0 }
func (m *sessionMockConnMgr) ClearConn()                {}
func (m *sessionMockConnMgr) GetAllConnID() []uint64    { return nil }
func (m *sessionMockConnMgr) GetAllConnIdStr() []string { return nil }
func (m *sessionMockConnMgr) Range(fn func(uint64, ziface.IConnection, interface{}) error, extra interface{}) error {
	for _, c := range m.conns {
		fn(0, c, extra)
	}
	return nil
}
func (m *sessionMockConnMgr) Range2(func(string, ziface.IConnection, interface{}) error, interface{}) error {
	return nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

func clearSessions() {
	sessions.Range(func(k, _ interface{}) bool {
		sessions.Delete(k)
		return true
	})
}

func sessionData(t *testing.T, playerID int64, gatewayID string, disconnectedAt int64, tags map[string]string) []byte {
	t.Helper()
	b, _ := proto.Marshal(&pb.SessionData{
		PlayerId:       playerID,
		GatewayId:      gatewayID,
		DisconnectedAt: disconnectedAt,
		ConnTags:       tags,
	})
	return b
}

// ── Bug #1: handleGet always responds ───────────────────────────────────────

func TestHandleGet_NoSession_AlwaysResponds(t *testing.T) {
	clearSessions()

	sr := &SessionRouter{Reg: &pkg.Registry{}}
	conn := &sessionMockConn{}
	req := &sessionMockRequest{
		msgID: protocol.MsgIdSessionGet,
		data:  sessionData(t, 42, "", 0, nil),
		conn:  conn,
	}

	sr.handleGet(req)

	msgID, data := conn.lastMsg()
	if msgID != protocol.MsgIdSessionGet {
		t.Fatalf("expected response msgID %d, got %d", protocol.MsgIdSessionGet, msgID)
	}
	if data == nil {
		t.Fatal("expected response data, got nil")
	}

	var resp pb.SessionData
	if err := proto.Unmarshal(data, &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.PlayerId != 42 {
		t.Errorf("expected PlayerId=42, got %d", resp.PlayerId)
	}
	// New player: no gateway, no tags, DisconnectedAt=0 (zero value)
	if resp.GatewayId != "" {
		t.Errorf("expected empty GatewayId for new player, got %s", resp.GatewayId)
	}
	if resp.DisconnectedAt != 0 {
		t.Errorf("expected DisconnectedAt=0 for new player, got %d", resp.DisconnectedAt)
	}
}

func TestHandleGet_ExistingSession_ReturnsData(t *testing.T) {
	clearSessions()

	// Store an existing session
	const playerID int64 = 99
	sessions.Store(playerID, &Session{
		PlayerID:       playerID,
		GatewayID:      "gw-1",
		DisconnectedAt: 1234567890,
		ConnTags:       map[string]string{"room_server_id": "roomsvr-1"},
	})

	sr := &SessionRouter{Reg: &pkg.Registry{}}
	conn := &sessionMockConn{}
	req := &sessionMockRequest{
		msgID: protocol.MsgIdSessionGet,
		data:  sessionData(t, playerID, "", 0, nil),
		conn:  conn,
	}

	sr.handleGet(req)

	_, data := conn.lastMsg()
	if data == nil {
		t.Fatal("expected response data")
	}

	var resp pb.SessionData
	if err := proto.Unmarshal(data, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.PlayerId != playerID {
		t.Errorf("PlayerId: got %d, want %d", resp.PlayerId, playerID)
	}
	if resp.GatewayId != "gw-1" {
		t.Errorf("GatewayId: got %s, want gw-1", resp.GatewayId)
	}
	if resp.DisconnectedAt != 1234567890 {
		t.Errorf("DisconnectedAt: got %d, want 1234567890", resp.DisconnectedAt)
	}
	if resp.ConnTags["room_server_id"] != "roomsvr-1" {
		t.Errorf("ConnTags: got %v, want room_server_id=roomsvr-1", resp.ConnTags)
	}
}

// ── Bug #2: handleDisconnect ignores stale gateway ─────────────────────────

func TestHandleDisconnect_StaleGateway_Ignored(t *testing.T) {
	clearSessions()

	const playerID int64 = 7
	// Session already points to gw-2 (player already reconnected elsewhere)
	sessions.Store(playerID, &Session{
		PlayerID:       playerID,
		GatewayID:      "gw-2",
		DisconnectedAt: 0, // connected to gw-2
		ConnTags:       map[string]string{"room_server_id": "roomsvr-1"},
	})

	sr := &SessionRouter{Reg: &pkg.Registry{}}
	req := &sessionMockRequest{
		msgID: protocol.MsgIdSessionDisconnect,
		// gw-1 sends a disconnect (stale)
		data: sessionData(t, playerID, "gw-1", 0, nil),
	}
	sr.handleDisconnect(req)

	// Session should still be connected to gw-2, DisconnectedAt unchanged
	v, ok := sessions.Load(playerID)
	if !ok {
		t.Fatal("session should still exist")
	}
	s := v.(*Session)
	if s.GatewayID != "gw-2" {
		t.Errorf("GatewayID should remain gw-2, got %s", s.GatewayID)
	}
	if s.DisconnectedAt != 0 {
		t.Errorf("DisconnectedAt should remain 0, got %d", s.DisconnectedAt)
	}
}

func TestHandleDisconnect_Normal_SetsDisconnected(t *testing.T) {
	clearSessions()

	const playerID int64 = 7
	sessions.Store(playerID, &Session{
		PlayerID:  playerID,
		GatewayID: "gw-1",
	})

	sr := &SessionRouter{Reg: &pkg.Registry{}}
	req := &sessionMockRequest{
		msgID: protocol.MsgIdSessionDisconnect,
		data:  sessionData(t, playerID, "gw-1", 0, nil),
	}
	sr.handleDisconnect(req)

	v, ok := sessions.Load(playerID)
	if !ok {
		t.Fatal("session should still exist")
	}
	s := v.(*Session)
	if s.DisconnectedAt == 0 {
		t.Error("DisconnectedAt should be set (> 0)")
	}
}

// ── Bug #2: handleReconnect broadcasts invalidation ────────────────────────

func TestHandleReconnect_SameGateway_NoInvalidation(t *testing.T) {
	clearSessions()

	const playerID int64 = 7
	sessions.Store(playerID, &Session{
		PlayerID:       playerID,
		GatewayID:      "gw-1",
		DisconnectedAt: 1234567890,
	})

	// Mock server with one gateway connection
	gwConn := &sessionMockConn{}
	mgr := &sessionMockConnMgr{conns: []ziface.IConnection{gwConn}}

	sr := &SessionRouter{
		Reg:    &pkg.Registry{},
		Server: &sessionMockServer{connMgr: mgr},
	}
	req := &sessionMockRequest{
		msgID: protocol.MsgIdSessionReconnect,
		data:  sessionData(t, playerID, "gw-1", 0, nil), // same gateway
		conn:  &sessionMockConn{},                       // new gateway's conn
	}

	sr.handleReconnect(req)

	// No invalidation should be sent (same gateway)
	msgID, _ := gwConn.lastMsg()
	if msgID == protocol.MsgIdSessionInvalidate {
		t.Error("expected NO invalidation broadcast when reconnecting to same gateway")
	}

	// Session should be updated
	v, ok := sessions.Load(playerID)
	if !ok {
		t.Fatal("session should exist")
	}
	s := v.(*Session)
	if s.DisconnectedAt != 0 {
		t.Errorf("DisconnectedAt should be 0 after reconnect, got %d", s.DisconnectedAt)
	}
}

func TestHandleReconnect_DifferentGateway_BroadcastsInvalidation(t *testing.T) {
	clearSessions()

	const playerID int64 = 7
	sessions.Store(playerID, &Session{
		PlayerID:       playerID,
		GatewayID:      "gw-old",
		DisconnectedAt: 1234567890,
	})

	// Two gateway connections in the mock server
	gwOldConn := &sessionMockConn{} // represents gw-old's connection to SessionSvr
	gwNewConn := &sessionMockConn{} // represents gw-new's connection to SessionSvr
	mgr := &sessionMockConnMgr{conns: []ziface.IConnection{gwOldConn, gwNewConn}}

	sr := &SessionRouter{
		Reg:    &pkg.Registry{},
		Server: &sessionMockServer{connMgr: mgr},
	}
	// The request connection is the new gateway's connection to SessionSvr
	req := &sessionMockRequest{
		msgID: protocol.MsgIdSessionReconnect,
		data:  sessionData(t, playerID, "gw-new", 0, nil),
		conn:  gwNewConn,
	}

	sr.handleReconnect(req)

	// Invalidation should have been sent to ALL gateway connections
	msgID, data := gwOldConn.lastMsg()
	if msgID != protocol.MsgIdSessionInvalidate {
		t.Errorf("expected invalidation msgID %d on old gateway conn, got %d",
			protocol.MsgIdSessionInvalidate, msgID)
	}
	if data == nil {
		t.Fatal("expected invalidation data")
	}
	var inv pb.SessionData
	if err := proto.Unmarshal(data, &inv); err != nil {
		t.Fatalf("unmarshal invalidation: %v", err)
	}
	if inv.PlayerId != playerID {
		t.Errorf("invalidation PlayerId: got %d, want %d", inv.PlayerId, playerID)
	}
	if inv.GatewayId != "gw-old" {
		t.Errorf("invalidation GatewayId: got %s, want gw-old", inv.GatewayId)
	}

	// The new gateway connection also received invalidation (it will ignore via GatewayId mismatch)
	msgID, _ = gwNewConn.lastMsg()
	if msgID != protocol.MsgIdSessionInvalidate {
		t.Errorf("expected invalidation also on new gateway conn, got %d", msgID)
	}

	// Session should be updated to gw-new
	v, ok := sessions.Load(playerID)
	if !ok {
		t.Fatal("session should exist")
	}
	s := v.(*Session)
	if s.GatewayID != "gw-new" {
		t.Errorf("GatewayID: got %s, want gw-new", s.GatewayID)
	}
	if s.DisconnectedAt != 0 {
		t.Errorf("DisconnectedAt should be 0 after reconnect, got %d", s.DisconnectedAt)
	}
}

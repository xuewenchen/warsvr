package gateway

import (
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

// ── mock types ────────────────────────────────────────────────────────────

type mockRequest struct {
	msgID uint32
	data  []byte
}

func (m *mockRequest) GetConnection() ziface.IConnection       { return nil }
func (m *mockRequest) GetData() []byte                         { return m.data }
func (m *mockRequest) GetMsgID() uint32                        { return m.msgID }
func (m *mockRequest) GetMessage() ziface.IMessage             { return nil }
func (m *mockRequest) GetResponse() ziface.IcResp              { return nil }
func (m *mockRequest) SetResponse(ziface.IcResp)               {}
func (m *mockRequest) BindRouter(ziface.IRouter)               {}
func (m *mockRequest) Call()                                   {}
func (m *mockRequest) Abort()                                  {}
func (m *mockRequest) Goto(ziface.HandleStep)                  {}
func (m *mockRequest) BindRouterSlices([]ziface.RouterHandler) {}
func (m *mockRequest) RouterSlicesNext()                       {}
func (m *mockRequest) Copy() ziface.IRequest                   { return m }
func (m *mockRequest) Set(string, interface{})                 {}
func (m *mockRequest) Get(string) (interface{}, bool)          { return nil, false }

type mockConn struct {
	connID    uint64
	stopCount int
	mu        sync.Mutex
	props     map[string]interface{}
}

func (m *mockConn) Start()                                                    {}
func (m *mockConn) Stop()                                                     { m.mu.Lock(); m.stopCount++; m.mu.Unlock() }
func (m *mockConn) Context() context.Context                                  { return nil }
func (m *mockConn) GetName() string                                           { return "" }
func (m *mockConn) GetConnection() net.Conn                                   { return nil }
func (m *mockConn) GetWsConn() *websocket.Conn                                { return nil }
func (m *mockConn) GetTCPConnection() net.Conn                                { return nil }
func (m *mockConn) GetConnID() uint64                                         { return m.connID }
func (m *mockConn) GetConnIdStr() string                                      { return "" }
func (m *mockConn) GetMsgHandler() ziface.IMsgHandle                          { return nil }
func (m *mockConn) GetWorkerID() uint32                                       { return 0 }
func (m *mockConn) RemoteAddr() net.Addr                                      { return nil }
func (m *mockConn) LocalAddr() net.Addr                                       { return nil }
func (m *mockConn) LocalAddrString() string                                   { return "" }
func (m *mockConn) RemoteAddrString() string                                  { return "" }
func (m *mockConn) Send([]byte) error                                         { return nil }
func (m *mockConn) SendToQueue([]byte, ...ziface.MsgSendOption) error         { return nil }
func (m *mockConn) SendMsg(uint32, []byte) error                              { return nil }
func (m *mockConn) SendBuffMsg(uint32, []byte, ...ziface.MsgSendOption) error { return nil }
func (m *mockConn) SetProperty(k string, v interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.props == nil {
		m.props = make(map[string]interface{})
	}
	m.props[k] = v
}
func (m *mockConn) GetProperty(k string) (interface{}, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if v, ok := m.props[k]; ok {
		return v, nil
	}
	return nil, nil // not using the error return convention of zinx
}
func (m *mockConn) RemoveProperty(string)                             {}
func (m *mockConn) IsAlive() bool                                     { return true }
func (m *mockConn) SetHeartBeat(ziface.IHeartbeatChecker)             {}
func (m *mockConn) AddCloseCallback(interface{}, interface{}, func()) {}
func (m *mockConn) RemoveCloseCallback(interface{}, interface{})      {}
func (m *mockConn) InvokeCloseCallbacks()                             {}

type mockConnMgr struct {
	conns map[uint64]ziface.IConnection
}

func (m *mockConnMgr) Add(ziface.IConnection)    { panic("unexpected") }
func (m *mockConnMgr) Remove(ziface.IConnection) { panic("unexpected") }
func (m *mockConnMgr) Get(id uint64) (ziface.IConnection, error) {
	if c, ok := m.conns[id]; ok {
		return c, nil
	}
	return nil, errors.New("connection not found")
}
func (m *mockConnMgr) Get2(string) (ziface.IConnection, error) { return nil, nil }
func (m *mockConnMgr) Len() int                                { return 0 }
func (m *mockConnMgr) ClearConn()                              {}
func (m *mockConnMgr) GetAllConnID() []uint64                  { return nil }
func (m *mockConnMgr) GetAllConnIdStr() []string               { return nil }
func (m *mockConnMgr) Range(func(uint64, ziface.IConnection, interface{}) error, interface{}) error {
	return nil
}
func (m *mockConnMgr) Range2(func(string, ziface.IConnection, interface{}) error, interface{}) error {
	return nil
}

type mockServer struct {
	connMgr ziface.IConnManager
}

func (m *mockServer) Start()                           {}
func (m *mockServer) Stop()                            {}
func (m *mockServer) Serve()                           {}
func (m *mockServer) AddRouter(uint32, ziface.IRouter) {}
func (m *mockServer) AddRouterSlices(uint32, ...ziface.RouterHandler) ziface.IRouterSlices {
	return nil
}
func (m *mockServer) Group(uint32, uint32, ...ziface.RouterHandler) ziface.IGroupRouterSlices {
	return nil
}
func (m *mockServer) Use(...ziface.RouterHandler) ziface.IRouterSlices                { return nil }
func (m *mockServer) GetConnMgr() ziface.IConnManager                                 { return m.connMgr }
func (m *mockServer) SetOnConnStart(func(ziface.IConnection))                         {}
func (m *mockServer) SetOnConnStop(func(ziface.IConnection))                          {}
func (m *mockServer) GetOnConnStart() func(ziface.IConnection)                        { return nil }
func (m *mockServer) GetOnConnStop() func(ziface.IConnection)                         { return nil }
func (m *mockServer) GetPacket() ziface.IDataPack                                     { return nil }
func (m *mockServer) GetMsgHandler() ziface.IMsgHandle                                { return nil }
func (m *mockServer) SetPacket(ziface.IDataPack)                                      {}
func (m *mockServer) StartHeartBeat(time.Duration)                                    {}
func (m *mockServer) StartHeartBeatWithOption(time.Duration, *ziface.HeartBeatOption) {}
func (m *mockServer) GetHeartBeat() ziface.IHeartbeatChecker                          { return nil }
func (m *mockServer) GetLengthField() *ziface.LengthField                             { return nil }
func (m *mockServer) SetDecoder(ziface.IDecoder)                                      {}
func (m *mockServer) AddInterceptor(ziface.IInterceptor)                              {}
func (m *mockServer) SetWebsocketAuth(func(*http.Request) error)                      {}
func (m *mockServer) ServerName() string                                              { return "mock" }

// ── helpers ──────────────────────────────────────────────────────────────────

func sessionDataBytes(t *testing.T, playerID int64, gatewayID string, disconnectedAt int64) []byte {
	t.Helper()
	b, _ := proto.Marshal(&pb.SessionData{
		PlayerId:       playerID,
		GatewayId:      gatewayID,
		DisconnectedAt: disconnectedAt,
	})
	return b
}

// ── Bug #1 tests: HandleSessionGet channel signaling ─────────────────────────

func TestHandleSessionGet_SignalsPendingChannel(t *testing.T) {
	const playerID int64 = 42
	ch := make(chan struct{})
	gw := &GatewayServer{
		ID:          "gw-1",
		PlayerConns: &sync.Map{},
	}
	gw.pendingSessionGets.Store(playerID, ch)

	req := &mockRequest{data: sessionDataBytes(t, playerID, "", 0)}
	gw.HandleSessionGet(req)

	select {
	case <-ch:
		// channel was closed — CheckReconnect unblocked ✓
	default:
		t.Fatal("expected channel to be closed after HandleSessionGet")
	}
}

func TestHandleSessionGet_NoPendingChannel_DoesNotPanic(t *testing.T) {
	const playerID int64 = 42
	gw := &GatewayServer{
		ID:          "gw-1",
		PlayerConns: &sync.Map{},
	}
	// no pendingSessionGets entry — typical timeout or non-reconnect path

	req := &mockRequest{data: sessionDataBytes(t, playerID, "", 0)}
	// should not panic
	gw.HandleSessionGet(req)
}

func TestHandleSessionGet_CheckReconnectRoundTrip(t *testing.T) {
	const playerID int64 = 99

	gw := &GatewayServer{
		ID:          "gw-1",
		PlayerConns: &sync.Map{},
	}

	// Simulate synchronous CheckReconnect without a real SessionSvr:
	// store the channel, send a response in a goroutine.
	ch := make(chan struct{})
	gw.pendingSessionGets.Store(playerID, ch)

	go func() {
		req := &mockRequest{data: sessionDataBytes(t, playerID, "", 0)}
		gw.HandleSessionGet(req)
	}()

	// Simulate CheckReconnect's select
	select {
	case <-ch:
		// success — HandleSessionGet signaled
	case <-time.After(time.Second):
		t.Fatal("CheckReconnect simulation timed out waiting for signal")
	}
}

// ── Bug #2 tests: HandleSessionInvalidate ─────────────────────────────────────

func TestHandleSessionInvalidate_MatchingGateway(t *testing.T) {
	const playerID int64 = 7
	const connID uint64 = 100

	conn := &mockConn{connID: connID}
	connMgr := &mockConnMgr{conns: map[uint64]ziface.IConnection{connID: conn}}

	gw := &GatewayServer{
		ID:          "gw-old",
		PlayerConns: &sync.Map{},
		Server:      &mockServer{connMgr: connMgr},
	}
	gw.PlayerConns.Store(playerID, connID)

	data, _ := proto.Marshal(&pb.SessionData{PlayerId: playerID, GatewayId: "gw-old"})
	req := &mockRequest{data: data}
	gw.HandleSessionInvalidate(req)

	// PlayerConns must be cleaned
	if _, exists := gw.PlayerConns.Load(playerID); exists {
		t.Error("expected player to be removed from PlayerConns")
	}

	// Connection must be stopped
	conn.mu.Lock()
	stops := conn.stopCount
	conn.mu.Unlock()
	if stops != 1 {
		t.Errorf("expected conn.Stop() to be called once, got %d", stops)
	}
}

func TestHandleSessionInvalidate_NonMatchingGateway(t *testing.T) {
	const playerID int64 = 7

	gw := &GatewayServer{
		ID:          "gw-new",
		PlayerConns: &sync.Map{},
	}
	gw.PlayerConns.Store(playerID, uint64(100))

	// invalidation targets "gw-old", but we are "gw-new"
	data, _ := proto.Marshal(&pb.SessionData{PlayerId: playerID, GatewayId: "gw-old"})
	req := &mockRequest{data: data}
	gw.HandleSessionInvalidate(req)

	// PlayerConns must NOT be affected
	if _, exists := gw.PlayerConns.Load(playerID); !exists {
		t.Error("expected player to remain in PlayerConns (non-matching gateway)")
	}
}

func TestHandleSessionInvalidate_PlayerNotInConns(t *testing.T) {
	gw := &GatewayServer{
		ID:          "gw-old",
		PlayerConns: &sync.Map{},
	}

	// player not in PlayerConns — should not panic
	data, _ := proto.Marshal(&pb.SessionData{PlayerId: 999, GatewayId: "gw-old"})
	req := &mockRequest{data: data}
	gw.HandleSessionInvalidate(req)
	// no panic = pass
}

func TestHandleSessionInvalidate_ConnAlreadyGone(t *testing.T) {
	const playerID int64 = 7
	const connID uint64 = 100

	// ConnMgr doesn't have the connection — simulates already-cleaned-up scenario
	connMgr := &mockConnMgr{conns: map[uint64]ziface.IConnection{}}

	gw := &GatewayServer{
		ID:          "gw-old",
		PlayerConns: &sync.Map{},
		Server:      &mockServer{connMgr: connMgr},
	}
	gw.PlayerConns.Store(playerID, connID)

	data, _ := proto.Marshal(&pb.SessionData{PlayerId: playerID, GatewayId: "gw-old"})
	req := &mockRequest{data: data}
	gw.HandleSessionInvalidate(req)

	// PlayerConns should still be cleaned even if ConnMgr.Get fails
	if _, exists := gw.PlayerConns.Load(playerID); exists {
		t.Error("expected player to be removed from PlayerConns even when conn is gone")
	}
}

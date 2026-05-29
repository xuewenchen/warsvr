package pkg

import (
	"cardwar/pkg/conf"
	"cardwar/protocol"
	"cardwar/protocol/pb"
	"sync"
	"testing"
	"time"

	"github.com/aceld/zinx/zconf"
	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
	"google.golang.org/protobuf/proto"
)

var portSeq = 20000

func nextPort() int { portSeq++; return portSeq }

type receivedMsg struct {
	msgID uint32
	data  []byte
}

type testRecvRouter struct {
	znet.BaseRouter
	ch chan receivedMsg
}

func (r *testRecvRouter) Handle(req ziface.IRequest) {
	r.ch <- receivedMsg{req.GetMsgID(), req.GetData()}
}

func startTestServer(t *testing.T, port int) ziface.IServer {
	t.Helper()
	s := NewServer(&zconf.Config{
		Name: "test-backend", Host: "127.0.0.1", TCPPort: port, Mode: zconf.ServerModeTcp,
	})
	s.Start()
	time.Sleep(100 * time.Millisecond)
	return s
}

func connectGateway(t *testing.T, port int, ch chan receivedMsg) ziface.IConnection {
	t.Helper()
	client := znet.NewClient("127.0.0.1", port)
	client.AddRouter(protocol.MsgIdChatResp, &testRecvRouter{ch: ch})
	connCh := make(chan ziface.IConnection, 1)
	client.SetOnConnStart(func(conn ziface.IConnection) {
		conn.SendMsg(protocol.MsgIdServiceIdentity, []byte(conf.SvcGateway))
		connCh <- conn
	})
	client.Start()
	select {
	case conn := <-connCh:
		return conn
	case <-time.After(3 * time.Second):
		t.Fatal("timeout connecting gateway client")
	}
	time.Sleep(50 * time.Millisecond) // let server process registration message
	return nil
}

func TestBroadcaster_ToAll(t *testing.T) {
	port := nextPort()
	srv := startTestServer(t, port)
	defer srv.Stop()

	ch := make(chan receivedMsg, 100)
	connectGateway(t, port, ch)
	bc := NewGateWayBroadcaster(srv)

	payload := []byte("hello broadcast")
	bc.ToAll(protocol.MsgIdChatResp, payload)

	select {
	case msg := <-ch:
		if msg.msgID != protocol.MsgIdChatResp {
			t.Errorf("expected msgID %d, got %d", protocol.MsgIdChatResp, msg.msgID)
		}
		env := &pb.Envelope{}
		proto.Unmarshal(msg.data, env)
		if env.ConnId != 0 {
			t.Errorf("expected ConnId=0 (broadcast), got %d", env.ConnId)
		}
		if string(env.Data) != string(payload) {
			t.Errorf("expected payload '%s', got '%s'", payload, env.Data)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for broadcast message")
	}
}

func TestBroadcaster_ToPlayer(t *testing.T) {
	port := nextPort()
	srv := startTestServer(t, port)
	defer srv.Stop()

	ch := make(chan receivedMsg, 100)
	connectGateway(t, port, ch)
	bc := NewGateWayBroadcaster(srv)

	payload := []byte("private msg")
	bc.ToPlayer(protocol.MsgIdChatResp, 42, payload)

	select {
	case msg := <-ch:
		env := &pb.Envelope{}
		proto.Unmarshal(msg.data, env)
		if env.ConnId != 0 {
			t.Error("expected ConnId=0")
		}
		target := env.ConnTags["target_player_id"]
		if target != "42" {
			t.Errorf("expected target_player_id=42, got %s", target)
		}
		if string(env.Data) != string(payload) {
			t.Errorf("expected payload '%s', got '%s'", payload, env.Data)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for player message")
	}
}

func TestBroadcaster_ToConn(t *testing.T) {
	port := nextPort()
	srv := startTestServer(t, port)
	defer srv.Stop()

	ch := make(chan receivedMsg, 100)
	connectGateway(t, port, ch)
	time.Sleep(200 * time.Millisecond) // let server register the connection
	bc := NewGateWayBroadcaster(srv)

	// Get the server-side connection from ConnMgr (this is what Broadcaster uses)
	var serverConn ziface.IConnection
	srv.GetConnMgr().Range(func(connID uint64, conn ziface.IConnection, extra interface{}) error {
		serverConn = conn
		return nil
	}, nil)

	payload := []byte("direct msg")
	bc.ToConn(protocol.MsgIdChatResp, 77, payload, serverConn)

	select {
	case msg := <-ch:
		env := &pb.Envelope{}
		proto.Unmarshal(msg.data, env)
		if env.ConnId != 77 {
			t.Errorf("expected connId 77, got %d", env.ConnId)
		}
		if string(env.Data) != string(payload) {
			t.Errorf("expected '%s', got '%s'", payload, env.Data)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for ToConn message")
	}
}

func TestBroadcaster_NonGatewayIgnored(t *testing.T) {
	port := nextPort()
	srv := startTestServer(t, port)
	defer srv.Stop()

	// Connect WITHOUT registering as gateway and WITHOUT a router
	client := znet.NewClient("127.0.0.1", port)
	connCh := make(chan ziface.IConnection, 1)
	client.SetOnConnStart(func(conn ziface.IConnection) { connCh <- conn })
	client.Start()
	var nonGWConn ziface.IConnection
	select {
	case nonGWConn = <-connCh:
	case <-time.After(3 * time.Second):
		t.Fatal("timeout")
	}
	defer nonGWConn.Stop()

	bc := NewGateWayBroadcaster(srv)
	// Should not panic or send to non-gateway connections
	bc.ToAll(protocol.MsgIdChatResp, []byte("should-not-receive"))
	time.Sleep(200 * time.Millisecond)
}

func TestBroadcaster_MultipleGateways(t *testing.T) {
	port := nextPort()
	srv := startTestServer(t, port)
	defer srv.Stop()

	ch := make(chan receivedMsg, 100)
	n := 3
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			connectGateway(t, port, ch)
		}()
	}
	wg.Wait()
	time.Sleep(200 * time.Millisecond) // let server process all registrations

	bc := NewGateWayBroadcaster(srv)
	bc.ToAll(protocol.MsgIdChatResp, []byte("multi-gw"))

	received := 0
	timeout := time.After(2 * time.Second)
	for i := 0; i < n; i++ {
		select {
		case <-ch:
			received++
		case <-timeout:
			t.Fatalf("expected %d messages, got %d", n, received)
		}
	}
}

func BenchmarkBroadcastTo_1Gateway(b *testing.B) {
	port := nextPort()
	srv := startTestServerB(b, port)
	defer srv.Stop()

	ch := make(chan receivedMsg, 1000)
	connectGatewayB(b, port, ch)
	bc := NewGateWayBroadcaster(srv)
	payload := []byte("benchmark")

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		bc.ToAll(protocol.MsgIdChatResp, payload)
		<-ch
	}
}

func BenchmarkBroadcastTo_10Gateways(b *testing.B) {
	port := nextPort()
	srv := startTestServerB(b, port)
	defer srv.Stop()

	ch := make(chan receivedMsg, 1000)
	for i := 0; i < 10; i++ {
		connectGatewayB(b, port, ch)
	}
	bc := NewGateWayBroadcaster(srv)
	payload := []byte("benchmark")

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		bc.ToAll(protocol.MsgIdChatResp, payload)
		for j := 0; j < 10; j++ {
			<-ch
		}
	}
}

func startTestServerB(b *testing.B, port int) ziface.IServer {
	s := NewServer(&zconf.Config{
		Name: "test-backend", Host: "127.0.0.1", TCPPort: port, Mode: zconf.ServerModeTcp,
	})
	s.Start()
	time.Sleep(100 * time.Millisecond)
	return s
}

func connectGatewayB(b *testing.B, port int, ch chan receivedMsg) ziface.IConnection {
	client := znet.NewClient("127.0.0.1", port)
	client.AddRouter(protocol.MsgIdChatResp, &testRecvRouter{ch: ch})
	connCh := make(chan ziface.IConnection, 1)
	client.SetOnConnStart(func(conn ziface.IConnection) {
		conn.SendMsg(protocol.MsgIdServiceIdentity, []byte(conf.SvcGateway))
		connCh <- conn
	})
	client.Start()
	select {
	case conn := <-connCh:
		return conn
	case <-time.After(3 * time.Second):
		b.Fatal("timeout connecting gateway client")
	}
	return nil
}

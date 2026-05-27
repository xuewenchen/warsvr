package e2e

import (
	"cardwar/pkg/auth"
	"cardwar/protocol"
	"cardwar/protocol/pb"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
	"google.golang.org/protobuf/proto"
)

const jwtSecret = "e2e-test-secret"

var (
	chatsvrBin string
	gatewayBin string
	portBase   atomic.Int32
)

func init() { portBase.Store(21000) }

func nextPort() int { return int(portBase.Add(1)) }

func TestMain(m *testing.M) {
	root, err := findProjectRoot()
	if err != nil {
		fmt.Println("find root:", err)
		os.Exit(1)
	}

	tmpDir, err := os.MkdirTemp("", "e2e-*")
	if err != nil {
		fmt.Println("temp dir:", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	chatsvrBin = filepath.Join(tmpDir, "chatsvr.exe")
	gatewayBin = filepath.Join(tmpDir, "gateway.exe")

	build := func(outPath, pkg string) {
		cmd := exec.Command("go", "build", "-o", outPath, pkg)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			fmt.Printf("build %s: %v\n%s\n", pkg, err, out)
			os.Exit(1)
		}
	}
	build(chatsvrBin, "./apps/chatsvr/cmd/")
	build(gatewayBin, "./apps/gateway/cmd/")

	os.Exit(m.Run())
}

// ------- topology helpers -------

type topoConfig struct {
	csPorts   []int
	gwTCPs    []int
	gwWSPorts []int
	confPath  string
}

// writeTopoConfig writes config.yml for the given topology.
func writeTopoConfig(t *testing.T, csPorts, gwTCPs, gwWSPorts []int) *topoConfig {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")

	var csLines, gwLines string
	for i, p := range csPorts {
		csLines += fmt.Sprintf("    - id: cs-%d\n      listen: 0.0.0.0:%d\n", i+1, p)
	}
	for i := range gwTCPs {
		gwLines += fmt.Sprintf("    - id: gw-%d\n      tcp_listen: 0.0.0.0:%d\n      ws_listen: 0.0.0.0:%d\n", i+1, gwTCPs[i], gwWSPorts[i])
	}

	content := fmt.Sprintf(`
services:
  gateway:
%s
  chatsvr:
%s
gateway:
  jwt_secret: "%s"
  routes:
    chatsvr:
      forward: [5]
      route_key: playerId
`, gwLines, csLines, jwtSecret)

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return &topoConfig{csPorts: csPorts, gwTCPs: gwTCPs, gwWSPorts: gwWSPorts, confPath: path}
}

func startProcess(t *testing.T, bin, confPath, id string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(bin, "-conf", confPath)
	if id != "" {
		cmd.Args = append(cmd.Args, "-id", id)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start %s: %v", bin, err)
	}
	t.Cleanup(func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
	})
	return cmd
}

func startChatSvrs(t *testing.T, topo *topoConfig) {
	for i := range topo.csPorts {
		id := fmt.Sprintf("cs-%d", i+1)
		startProcess(t, chatsvrBin, topo.confPath, id)
	}
	for _, p := range topo.csPorts {
		waitPort(t, p, 5*time.Second)
	}
}

func startGateways(t *testing.T, topo *topoConfig) {
	for i := range topo.gwTCPs {
		id := fmt.Sprintf("gw-%d", i+1)
		startProcess(t, gatewayBin, topo.confPath, id)
	}
	for _, p := range topo.gwWSPorts {
		waitPort(t, p, 10*time.Second)
	}
	time.Sleep(time.Duration(len(topo.csPorts)) * 500 * time.Millisecond) // let Gateways Dial all ChatSvr instances
}

func waitPort(t *testing.T, port int, d time.Duration) {
	t.Helper()
	deadline := time.After(d)
	for {
		select {
		case <-deadline:
			t.Fatalf("port %d not ready after %v", port, d)
		default:
			conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 100*time.Millisecond)
			if err == nil {
				conn.Close()
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// ------- test client -------

type testClient struct {
	playerID int64
	conn     ziface.IConnection
	recvCh   chan *pb.ChatResp
}

func connectClient(t *testing.T, playerID int64, wsPort int) *testClient {
	t.Helper()
	token, err := auth.GenerateJWT(playerID, jwtSecret)
	if err != nil {
		t.Fatalf("generate JWT: %v", err)
	}

	wsURL := &url.URL{
		Scheme:   "ws",
		Host:     fmt.Sprintf("127.0.0.1:%d", wsPort),
		Path:     "/ws",
		RawQuery: "token=" + url.QueryEscape(token),
	}
	client := znet.NewWsClient("127.0.0.1", wsPort, znet.WithUrl(wsURL))

	recvCh := make(chan *pb.ChatResp, 100)
	client.AddRouter(protocol.MsgIdPong, &pongRouter{})
	client.AddRouter(protocol.MsgIdChatResp, &chatRespRouter{ch: recvCh})

	connCh := make(chan ziface.IConnection, 1)
	client.SetOnConnStart(func(conn ziface.IConnection) { connCh <- conn })
	client.Start()

	select {
	case conn := <-connCh:
		t.Logf("player %d connected to ws:%d", playerID, wsPort)
		return &testClient{playerID: playerID, conn: conn, recvCh: recvCh}
	case <-time.After(5 * time.Second):
		t.Fatalf("player %d: timeout connecting to ws:%d", playerID, wsPort)
	}
	return nil
}

type pongRouter struct{ znet.BaseRouter }

func (r *pongRouter) Handle(req ziface.IRequest) {}

type chatRespRouter struct {
	znet.BaseRouter
	ch chan *pb.ChatResp
}

func (r *chatRespRouter) Handle(req ziface.IRequest) {
	var msg pb.ChatResp
	if err := proto.Unmarshal(req.GetData(), &msg); err != nil {
		return
	}
	r.ch <- &msg
}

func (c *testClient) sendChat(content string, target int64) {
	req := &pb.ChatReq{Content: content, TargetPlayerId: target}
	data, _ := proto.Marshal(req)
	c.conn.SendMsg(protocol.MsgIdChatReq, data)
}

func requireChatResp(t *testing.T, ch chan *pb.ChatResp, timeout time.Duration) *pb.ChatResp {
	t.Helper()
	select {
	case msg := <-ch:
		return msg
	case <-time.After(timeout):
		t.Fatal("timeout waiting for ChatResp")
	}
	return nil
}

func requireNoChatResp(t *testing.T, ch chan *pb.ChatResp, wait time.Duration) {
	t.Helper()
	select {
	case msg := <-ch:
		t.Fatalf("unexpected ChatResp: %+v", msg)
	case <-time.After(wait):
	}
}

// ------- topology tests -------

// TestE2E_MultiGW_SingleCS: 2 Gateways, 1 ChatSvr.
func TestE2E_MultiGW_SingleCS(t *testing.T) {
	csPorts := []int{nextPort()}
	gwTCPs := []int{nextPort(), nextPort()}
	gwWSPorts := []int{nextPort(), nextPort()}

	topo := writeTopoConfig(t, csPorts, gwTCPs, gwWSPorts)
	startChatSvrs(t, topo)
	startGateways(t, topo)

	alice := connectClient(t, 1, gwWSPorts[0]) // GW-1
	bob := connectClient(t, 2, gwWSPorts[1])   // GW-2
	time.Sleep(300 * time.Millisecond)

	// Global chat: Alice (GW-1) sends, both receive
	alice.sendChat("cross-gw global", 0)
	requireChatResp(t, alice.recvCh, 3*time.Second)
	requireChatResp(t, bob.recvCh, 3*time.Second)

	// Private chat: Alice (GW-1) → Bob (GW-2)
	alice.sendChat("cross-gw private", 2)
	requireChatResp(t, alice.recvCh, 3*time.Second) // confirmation
	msg := requireChatResp(t, bob.recvCh, 3*time.Second)
	if msg.SenderPlayerId != 1 || msg.Content != "cross-gw private" {
		t.Errorf("bob got: sender=%d content=%s", msg.SenderPlayerId, msg.Content)
	}
}

// TestE2E_SingleGW_MultiCS: 1 Gateway, 2 ChatSvr instances.
func TestE2E_SingleGW_MultiCS(t *testing.T) {
	csPorts := []int{nextPort(), nextPort()}
	gwTCPs := []int{nextPort()}
	gwWSPorts := []int{nextPort()}

	topo := writeTopoConfig(t, csPorts, gwTCPs, gwWSPorts)
	startChatSvrs(t, topo)
	startGateways(t, topo)

	alice := connectClient(t, 1, gwWSPorts[0])
	bob := connectClient(t, 2, gwWSPorts[0])
	charlie := connectClient(t, 3, gwWSPorts[0])
	time.Sleep(300 * time.Millisecond)

	// Global chat: all three receive
	alice.sendChat("multi-cs global", 0)
	requireChatResp(t, alice.recvCh, 3*time.Second)
	requireChatResp(t, bob.recvCh, 3*time.Second)
	requireChatResp(t, charlie.recvCh, 3*time.Second)

	// Private chat: Alice → Charlie
	alice.sendChat("multi-cs private", 3)
	requireChatResp(t, alice.recvCh, 3*time.Second) // confirmation
	msg := requireChatResp(t, charlie.recvCh, 3*time.Second)
	if msg.SenderPlayerId != 1 || msg.TargetPlayerId != 3 {
		t.Errorf("charlie got: sender=%d target=%d", msg.SenderPlayerId, msg.TargetPlayerId)
	}
	// Bob should NOT receive private message
	requireNoChatResp(t, bob.recvCh, 500*time.Millisecond)
}

// TestE2E_MultiGW_MultiCS: 2 Gateways, 2 ChatSvr instances.
func TestE2E_MultiGW_MultiCS(t *testing.T) {
	csPorts := []int{nextPort(), nextPort()}
	gwTCPs := []int{nextPort(), nextPort()}
	gwWSPorts := []int{nextPort(), nextPort()}

	topo := writeTopoConfig(t, csPorts, gwTCPs, gwWSPorts)
	startChatSvrs(t, topo)
	startGateways(t, topo)

	// Players spread across Gateways
	alice := connectClient(t, 1, gwWSPorts[0])   // GW-1
	bob := connectClient(t, 2, gwWSPorts[1])     // GW-2
	charlie := connectClient(t, 3, gwWSPorts[0]) // GW-1
	time.Sleep(300 * time.Millisecond)

	// Global chat from Alice (GW-1, CS determined by hash)
	alice.sendChat("full-mesh-global", 0)
	requireChatResp(t, alice.recvCh, 3*time.Second)
	requireChatResp(t, bob.recvCh, 3*time.Second)
	requireChatResp(t, charlie.recvCh, 3*time.Second)

	// Private: Charlie (GW-1) → Bob (GW-2)
	charlie.sendChat("cross-gw-cs-private", 2)
	requireChatResp(t, charlie.recvCh, 3*time.Second)
	msg := requireChatResp(t, bob.recvCh, 3*time.Second)
	if msg.SenderPlayerId != 3 || msg.Content != "cross-gw-cs-private" {
		t.Errorf("bob got: sender=%d content=%s", msg.SenderPlayerId, msg.Content)
	}

	// Private: Bob (GW-2) → Alice (GW-1)
	bob.sendChat("reverse-cross", 1)
	requireChatResp(t, bob.recvCh, 3*time.Second)
	msg = requireChatResp(t, alice.recvCh, 3*time.Second)
	if msg.SenderPlayerId != 2 || msg.Content != "reverse-cross" {
		t.Errorf("alice got: sender=%d content=%s", msg.SenderPlayerId, msg.Content)
	}
}

// TestE2E_ThreeGateways_OneChatSvr: 3 Gateways, 1 ChatSvr (stress test).
func TestE2E_ThreeGateways_OneChatSvr(t *testing.T) {
	csPorts := []int{nextPort()}
	n := 3
	gwTCPs := make([]int, n)
	gwWSPorts := make([]int, n)
	for i := 0; i < n; i++ {
		gwTCPs[i] = nextPort()
		gwWSPorts[i] = nextPort()
	}

	topo := writeTopoConfig(t, csPorts, gwTCPs, gwWSPorts)
	startChatSvrs(t, topo)
	startGateways(t, topo)

	// 1 player per Gateway
	var clients []*testClient
	for i := 0; i < n; i++ {
		c := connectClient(t, int64(i+1), gwWSPorts[i])
		clients = append(clients, c)
	}
	time.Sleep(300 * time.Millisecond)

	// Global chat from player on GW-1 — all N players receive
	clients[0].sendChat("3gw-global", 0)
	for _, c := range clients {
		requireChatResp(t, c.recvCh, 3*time.Second)
	}

	// Private: GW-1 player → GW-3 player
	clients[0].sendChat("3gw-private", int64(n))
	requireChatResp(t, clients[0].recvCh, 3*time.Second) // confirm
	msg := requireChatResp(t, clients[n-1].recvCh, 3*time.Second)
	if msg.SenderPlayerId != 1 || msg.TargetPlayerId != int64(n) {
		t.Errorf("got: sender=%d target=%d", msg.SenderPlayerId, msg.TargetPlayerId)
	}
}

// ------- reliability test -------

// TestE2E_NoMessageLoss sends exactly N messages and verifies the receiver
// gets every one, with no gaps and no duplicates.
func TestE2E_NoMessageLoss(t *testing.T) {
	csPorts := []int{nextPort()}
	gwTCPs := []int{nextPort()}
	gwWSPorts := []int{nextPort()}

	topo := writeTopoConfig(t, csPorts, gwTCPs, gwWSPorts)
	startChatSvrs(t, topo)
	startGateways(t, topo)

	const N = 1000

	sender := connectClient(t, 1, gwWSPorts[0])
	receiver := connectClient(t, 2, gwWSPorts[0])
	time.Sleep(300 * time.Millisecond)

	// Sender sends N global chat messages with sequential IDs
	go func() {
		for i := 0; i < N; i++ {
			msg := &pb.ChatReq{Content: fmt.Sprintf("%d", i)}
			data, _ := proto.Marshal(msg)
			sender.conn.SendMsg(protocol.MsgIdChatReq, data)
			time.Sleep(1 * time.Millisecond) // throttle slightly
		}
	}()

	// Receiver collects all messages from sender (player 1)
	received := make(map[int]bool)
	timeout := time.After(time.Duration(N/50+5) * time.Second)

	for len(received) < N {
		select {
		case msg := <-receiver.recvCh:
			if msg.SenderPlayerId != 1 {
				continue // ignore other sources
			}
			seq, err := strconv.Atoi(msg.Content)
			if err != nil {
				t.Errorf("unparseable content: %s", msg.Content)
				continue
			}
			if received[seq] {
				t.Errorf("duplicate message: seq=%d", seq)
			}
			received[seq] = true
		case <-timeout:
			t.Fatalf("timeout: received %d/%d messages", len(received), N)
		}
	}

	// Check for gaps
	for i := 0; i < N; i++ {
		if !received[i] {
			t.Errorf("missing message: seq=%d", i)
		}
	}

	// Also check sender got N confirmations back (for global chat, sender also receives)
	senderReceived := 0
drainLoop:
	for {
		select {
		case msg := <-sender.recvCh:
			if msg.SenderPlayerId == 1 {
				senderReceived++
			}
		default:
			break drainLoop
		}
	}
	t.Logf("sender received %d of its own messages back", senderReceived)
}

// TestE2E_NoMessageLoss_Private sends N private messages and verifies
// the target receives every one without loss.
func TestE2E_NoMessageLoss_Private(t *testing.T) {
	csPorts := []int{nextPort()}
	gwTCPs := []int{nextPort()}
	gwWSPorts := []int{nextPort()}

	topo := writeTopoConfig(t, csPorts, gwTCPs, gwWSPorts)
	startChatSvrs(t, topo)
	startGateways(t, topo)

	const N = 500

	sender := connectClient(t, 1, gwWSPorts[0])
	receiver := connectClient(t, 2, gwWSPorts[0])
	time.Sleep(300 * time.Millisecond)

	go func() {
		for i := 0; i < N; i++ {
			msg := &pb.ChatReq{Content: fmt.Sprintf("%d", i), TargetPlayerId: 2}
			data, _ := proto.Marshal(msg)
			sender.conn.SendMsg(protocol.MsgIdChatReq, data)
			time.Sleep(2 * time.Millisecond)
		}
	}()

	received := make(map[int]bool)
	timeout := time.After(time.Duration(N/20+5) * time.Second)

	for len(received) < N {
		select {
		case msg := <-receiver.recvCh:
			if msg.SenderPlayerId != 1 {
				continue
			}
			seq, _ := strconv.Atoi(msg.Content)
			if received[seq] {
				t.Errorf("duplicate private message: seq=%d", seq)
			}
			received[seq] = true
		case <-timeout:
			t.Fatalf("timeout: received %d/%d private messages", len(received), N)
		}
	}

	for i := 0; i < N; i++ {
		if !received[i] {
			t.Errorf("missing private message: seq=%d", i)
		}
	}
}

// ------- utils -------

func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found")
		}
		dir = parent
	}
}

// ------- benchmarks -------

// startBenchServers starts ChatSvr + Gateway once and returns the WS port.
func startBenchServers(b *testing.B) int {
	b.Helper()
	csP := nextPort()
	gwTCP := nextPort()
	gwWS := nextPort()

	dir := b.TempDir()
	confPath := filepath.Join(dir, "config.yml")
	content := fmt.Sprintf(`
services:
  gateway:
    - id: gw-1
      tcp_listen: 0.0.0.0:%d
      ws_listen: 0.0.0.0:%d
  chatsvr:
    - id: cs-1
      listen: 0.0.0.0:%d
gateway:
  jwt_secret: "%s"
  routes:
    chatsvr:
      forward: [5]
      route_key: playerId
`, gwTCP, gwWS, csP, jwtSecret)
	if err := os.WriteFile(confPath, []byte(content), 0644); err != nil {
		b.Fatal(err)
	}

	csCmd := exec.Command(chatsvrBin, "-conf", confPath, "-id", "cs-1")
	csCmd.Stdout = nil
	csCmd.Stderr = nil
	csCmd.Start()

	gwCmd := exec.Command(gatewayBin, "-conf", confPath, "-id", "gw-1")
	gwCmd.Stdout = nil
	gwCmd.Stderr = nil
	gwCmd.Start()

	b.Cleanup(func() {
		csCmd.Process.Kill()
		gwCmd.Process.Kill()
	})

	waitPortBench(b, csP, 5*time.Second)
	waitPortBench(b, gwWS, 10*time.Second)
	time.Sleep(500 * time.Millisecond)
	return gwWS
}

func waitPortBench(b *testing.B, port int, d time.Duration) {
	deadline := time.After(d)
	for {
		select {
		case <-deadline:
			b.Fatalf("port %d not ready", port)
		default:
			conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 100*time.Millisecond)
			if err == nil {
				conn.Close()
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func connectBenchClient(b *testing.B, playerID int64, wsPort int) *testClient {
	token, _ := auth.GenerateJWT(playerID, jwtSecret)
	wsURL := &url.URL{
		Scheme:   "ws",
		Host:     fmt.Sprintf("127.0.0.1:%d", wsPort),
		Path:     "/ws",
		RawQuery: "token=" + url.QueryEscape(token),
	}
	client := znet.NewWsClient("127.0.0.1", wsPort, znet.WithUrl(wsURL))

	recvCh := make(chan *pb.ChatResp, 10000)
	client.AddRouter(protocol.MsgIdChatResp, &chatRespRouter{ch: recvCh})

	connCh := make(chan ziface.IConnection, 1)
	client.SetOnConnStart(func(conn ziface.IConnection) { connCh <- conn })
	client.Start()

	select {
	case conn := <-connCh:
		return &testClient{playerID: playerID, conn: conn, recvCh: recvCh}
	case <-time.After(5 * time.Second):
		b.Fatal("bench client timeout")
	}
	return nil
}

// BenchmarkE2E_Latency measures end-to-end latency: send ChatReq → receive ChatResp.
func BenchmarkE2E_Latency(b *testing.B) {
	wsPort := startBenchServers(b)
	c := connectBenchClient(b, 1, wsPort)
	time.Sleep(200 * time.Millisecond)

	req := &pb.ChatReq{Content: "bench"}
	data, _ := proto.Marshal(req)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		c.conn.SendMsg(protocol.MsgIdChatReq, data)
		<-c.recvCh
	}
}

// BenchmarkE2E_Throughput measures throughput with N concurrent senders.
func BenchmarkE2E_Throughput(b *testing.B) {
	wsPort := startBenchServers(b)
	n := 10
	var clients []*testClient
	for i := 0; i < n; i++ {
		c := connectBenchClient(b, int64(i+1), wsPort)
		clients = append(clients, c)
	}
	time.Sleep(200 * time.Millisecond)

	req := &pb.ChatReq{Content: "bench"}
	data, _ := proto.Marshal(req)

	b.ReportAllocs()
	b.ResetTimer()
	b.SetParallelism(n)
	b.RunParallel(func(pb *testing.PB) {
		// Each goroutine uses its own client; pick by goroutine id approximation
		idx := 0
		for pb.Next() {
			clients[idx%n].conn.SendMsg(protocol.MsgIdChatReq, data)
			<-clients[idx%n].recvCh
			idx++
		}
	})
}

// BenchmarkE2E_BroadcastMass measures global broadcast with many connected clients.
func BenchmarkE2E_BroadcastMass(b *testing.B) {
	wsPort := startBenchServers(b)
	n := 50
	var clients []*testClient
	for i := 0; i < n; i++ {
		c := connectBenchClient(b, int64(i+1), wsPort)
		clients = append(clients, c)
	}
	time.Sleep(500 * time.Millisecond)

	req := &pb.ChatReq{Content: "mass"}
	data, _ := proto.Marshal(req)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		clients[0].conn.SendMsg(protocol.MsgIdChatReq, data)
		// Drain all N responses (global broadcast to everyone)
		for i := 0; i < n; i++ {
			<-clients[i].recvCh
		}
	}
}

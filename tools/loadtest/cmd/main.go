package main

import (
	"cardwar/pkg/auth"
	"cardwar/protocol"
	"cardwar/protocol/pb"
	"flag"
	"fmt"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
	"google.golang.org/protobuf/proto"
)

var (
	flagClients  = flag.Int("c", 10, "number of concurrent clients")
	flagDuration = flag.Int("d", 10, "test duration in seconds")
	flagHost     = flag.String("h", "127.0.0.1", "Gateway WebSocket host")
	flagPort     = flag.Int("p", 19000, "Gateway WebSocket port")
	flagSecret   = flag.String("secret", "change-me-in-production", "JWT secret")
	flagMode     = flag.String("mode", "global", "chat mode: global|private|mixed")
)

type stats struct {
	sent      atomic.Int64
	received  atomic.Int64
	errors    atomic.Int64
	latSum    atomic.Int64 // nanoseconds
	latencies []int64      // microsecond buckets for percentiles
}

func main() {
	flag.Parse()

	fmt.Printf("=== Chat Load Test ===\n")
	fmt.Printf("server:   ws://%s:%d/ws\n", *flagHost, *flagPort)
	fmt.Printf("clients:  %d\n", *flagClients)
	fmt.Printf("duration: %ds\n", *flagDuration)
	fmt.Printf("mode:     %s\n", *flagMode)
	fmt.Println()

	st := &stats{latencies: make([]int64, 0, 100000)}
	var wg sync.WaitGroup

	stopCh := make(chan struct{})
	startTime := time.Now()

	// Connect all clients
	fmt.Printf("Connecting %d clients...\n", *flagClients)
	for i := 0; i < *flagClients; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			c := connectClient(int64(id+1), &wg, st, stopCh)
			if c != nil {
				runClient(c, &wg, st, stopCh, int64(id+1))
			}
		}(i)
	}
	time.Sleep(2 * time.Second) // let connections settle

	fmt.Printf("Running for %ds...\n", *flagDuration)
	time.Sleep(time.Duration(*flagDuration) * time.Second)
	close(stopCh)
	wg.Wait()

	elapsed := time.Since(startTime)
	printReport(st, elapsed)
}

func connectClient(playerID int64, wg *sync.WaitGroup, st *stats, stopCh chan struct{}) *testClient {
	token, _ := auth.GenerateJWT(playerID, *flagSecret)
	wsURL := &url.URL{
		Scheme:   "ws",
		Host:     fmt.Sprintf("%s:%d", *flagHost, *flagPort),
		Path:     "/ws",
		RawQuery: "token=" + url.QueryEscape(token),
	}
	client := znet.NewWsClient(*flagHost, *flagPort, znet.WithUrl(wsURL))

	recvCh := make(chan *pb.ChatResp, 10000)
	client.AddRouter(protocol.MsgIdChatResp, &chatRespRouter{ch: recvCh})

	connCh := make(chan ziface.IConnection, 1)
	client.SetOnConnStart(func(conn ziface.IConnection) { connCh <- conn })
	client.Start()

	select {
	case conn := <-connCh:
		// Start receiver goroutine
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case rsp := <-recvCh:
					st.received.Add(1)
					st.latSum.Add(int64(time.Now().UnixNano() - int64(rsp.Timestamp)*int64(time.Millisecond/time.Nanosecond)))
				case <-stopCh:
					return
				}
			}
		}()
		return &testClient{playerID: playerID, conn: conn, recvCh: recvCh}
	case <-time.After(5 * time.Second):
		fmt.Printf("client %d: connection timeout\n", playerID)
		st.errors.Add(1)
		return nil
	}
}

func runClient(c *testClient, wg *sync.WaitGroup, st *stats, stopCh chan struct{}, myID int64) {
	var targetID int64
	if *flagMode == "private" || (*flagMode == "mixed" && myID%2 == 0) {
		// Send private to another player
		targetID = myID + 1
		if targetID > int64(*flagClients) {
			targetID = 1
		}
	}

	ticker := time.NewTicker(50 * time.Millisecond) // 20 msg/s per client
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			req := &pb.ChatReq{Content: fmt.Sprintf("msg-from-%d", myID), TargetPlayerId: targetID}
			data, _ := proto.Marshal(req)
			if err := c.conn.SendMsg(protocol.MsgIdChatReq, data); err != nil {
				st.errors.Add(1)
			}
			st.sent.Add(1)
		}
	}
}

func printReport(st *stats, elapsed time.Duration) {
	sent := st.sent.Load()
	recv := st.received.Load()
	errs := st.errors.Load()

	fmt.Println()
	fmt.Printf("=== Results ===\n")
	fmt.Printf("Duration:       %v\n", elapsed.Round(time.Millisecond))
	fmt.Printf("Sent:           %d\n", sent)
	fmt.Printf("Received:       %d\n", recv)
	fmt.Printf("Errors:         %d\n", errs)
	fmt.Printf("Throughput:     %.0f msg/s\n", float64(sent)/elapsed.Seconds())

	if recv > 0 {
		avgLat := time.Duration(st.latSum.Load() / recv)
		fmt.Printf("Avg latency:    %v\n", avgLat)
	}
}

// ------- minimal client types -------

type testClient struct {
	playerID int64
	conn     ziface.IConnection
	recvCh   chan *pb.ChatResp
}

type chatRespRouter struct {
	znet.BaseRouter
	ch chan *pb.ChatResp
}

func (r *chatRespRouter) Handle(req ziface.IRequest) {
	var msg pb.ChatResp
	if err := proto.Unmarshal(req.GetData(), &msg); err != nil {
		return
	}
	select {
	case r.ch <- &msg:
	default:
	}
}

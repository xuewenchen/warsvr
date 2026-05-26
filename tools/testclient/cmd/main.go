package main

import (
	"cardwar/protocol"
	"cardwar/protocol/pb"
	"cardwar/tools/testclient/cmd/router"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
	"google.golang.org/protobuf/proto"
)

var playerID string

func login(conn ziface.IConnection) {
	msg := &pb.LoginReq{PlayerId: playerID}
	data, _ := proto.Marshal(msg)
	conn.SendMsg(protocol.MsgIdLogin, data)
	fmt.Println("Sent login:", playerID)
}

func chatLoop(conn ziface.IConnection) {
	time.Sleep(2 * time.Second)
	for i := 0; ; i++ {
		msg := &pb.ChatReq{
			PlayerId: playerID,
			Content:  fmt.Sprintf("Hello #%d from %s", i, playerID),
		}
		data, _ := proto.Marshal(msg)
		if err := conn.SendMsg(protocol.MsgIdChat, data); err != nil {
			fmt.Println("Send chat error:", err)
			return
		}
		time.Sleep(5 * time.Second)
	}
}

func onClientStart(conn ziface.IConnection) {
	fmt.Println("Connected to gateway via WebSocket")
	login(conn)
	go chatLoop(conn)
}

func main() {
	if len(os.Args) > 1 {
		playerID = os.Args[1]
	} else {
		playerID = "player1"
	}

	wsURL := &url.URL{Scheme: "ws", Host: "127.0.0.1:9000", Path: "/ws"}
	client := znet.NewWsClient("127.0.0.1", 9000, znet.WithUrl(wsURL))
	client.SetOnConnStart(onClientStart)
	client.AddRouter(protocol.MsgIdPong, &router.PongRouter{})
	client.AddRouter(protocol.MsgIdBroadcast, &router.BroadcastRouter{})

	client.Start()
	select {}
}

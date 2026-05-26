package main

import (
	"cardwar/common"
	"cardwar/tools/testclient/cmd/router"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
)

var playerID string

func login(conn ziface.IConnection) {
	msg := common.LoginMsg{PlayerID: playerID}
	data, _ := json.Marshal(msg)
	conn.SendMsg(common.MsgIdLogin, data)
	fmt.Println("Sent login:", playerID)
}

func chatLoop(conn ziface.IConnection) {
	time.Sleep(2 * time.Second)
	for i := 0; ; i++ {
		msg := common.ChatMsg{
			PlayerID: playerID,
			Content:  fmt.Sprintf("Hello #%d from %s", i, playerID),
		}
		data, _ := json.Marshal(msg)
		if err := conn.SendMsg(common.MsgIdChat, data); err != nil {
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
	client.AddRouter(common.MsgIdPong, &router.PongRouter{})
	client.AddRouter(common.MsgIdBroadcast, &router.BroadcastRouter{})

	client.Start()
	select {}
}

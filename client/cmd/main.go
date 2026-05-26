package main

import (
	"cardwar/client/cmd/router"
	"cardwar/common"
	"encoding/json"
	"fmt"
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
	fmt.Println("Connected to gateway")
	login(conn)
	go chatLoop(conn)
}

func main() {
	if len(os.Args) > 1 {
		playerID = os.Args[1]
	} else {
		playerID = "player1"
	}

	client := znet.NewClient("127.0.0.1", 8999)
	client.SetOnConnStart(onClientStart)
	client.AddRouter(common.MsgIdPong, &router.PongRouter{})
	client.AddRouter(common.MsgIdBroadcast, &router.BroadcastRouter{})

	client.Start()
	select {}
}

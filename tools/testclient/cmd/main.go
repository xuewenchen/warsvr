package main

import (
	"cardwar/conf"
	"cardwar/pkg/auth"
	"cardwar/protocol"
	"cardwar/protocol/pb"
	"cardwar/tools/testclient/cmd/router"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
	"google.golang.org/protobuf/proto"
)

var playerID int64

func chatLoop(conn ziface.IConnection) {
	time.Sleep(2 * time.Second)
	for i := 0; ; i++ {
		msg := &pb.ChatReq{
			Content: fmt.Sprintf("Hello #%d from %d", i, playerID),
		}
		data, _ := proto.Marshal(msg)
		if err := conn.SendMsg(protocol.MsgIdChatReq, data); err != nil {
			fmt.Println("Send chat error:", err)
			return
		}
		time.Sleep(5 * time.Second)
	}
}

func onClientStart(conn ziface.IConnection) {
	fmt.Println("Connected to gateway via WebSocket")
	go chatLoop(conn)
}

func main() {
	pidStr := "1"
	if len(os.Args) > 1 {
		pidStr = os.Args[1]
	}
	playerID, _ = strconv.ParseInt(pidStr, 10, 64)

	if err := conf.Load("config.yml"); err != nil {
		fmt.Println("Failed to load config:", err)
		os.Exit(1)
	}

	if conf.GlobalConfig.Gateway.JWTSecret == "" {
		fmt.Println("JWT secret not configured")
		os.Exit(1)
	}

	token, err := auth.GenerateJWT(playerID, conf.GlobalConfig.Gateway.JWTSecret)
	if err != nil {
		fmt.Println("Failed to generate JWT:", err)
		os.Exit(1)
	}

	wsURL := &url.URL{
		Scheme:   "ws",
		Host:     "127.0.0.1:9000",
		Path:     "/ws",
		RawQuery: "token=" + url.QueryEscape(token),
	}
	client := znet.NewWsClient("127.0.0.1", 9000, znet.WithUrl(wsURL))
	client.SetOnConnStart(onClientStart)
	client.AddRouter(protocol.MsgIdPong, &router.PongRouter{})
	client.AddRouter(protocol.MsgIdChatResp, &router.ChatRespRouter{})

	client.Start()
	select {}
}

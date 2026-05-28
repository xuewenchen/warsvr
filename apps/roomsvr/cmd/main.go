package main

import (
	"cardwar/apps/roomsvr/internal/router"
	"cardwar/pkg"
	"cardwar/pkg/conf"
	"cardwar/protocol"
	"flag"

	"github.com/aceld/zinx/zconf"
	"github.com/aceld/zinx/znet"
)

func main() {
	configPath := flag.String("conf", "config.yml", "path to config file")
	svrID := flag.String("id", "", "RoomSvr ID (matches config services.roomsvr[].id)")
	flag.Parse()

	if err := conf.Load(*configPath); err != nil {
		panic(err)
	}

	cfg := conf.LookupServer(conf.GlobalConfig.Services["roomsvr"], *svrID, "RoomSvr")
	host, port := conf.ParseHostPort(cfg.Listen)

	s := znet.NewUserConfServer(&zconf.Config{
		Name:    "RoomSvr",
		Host:    host,
		TCPPort: port,
		Mode:    zconf.ServerModeTcp,
	})

	s.AddRouter(protocol.MsgIdPing, &router.PingRouter{})
	s.AddRouter(protocol.MsgIdGatewayRegister, &router.GatewayRegisterRouter{})
	s.AddRouter(protocol.MsgIdRoomJoinReq, &router.RoomRouter{BC: pkg.NewBroadcaster(s)})
	s.AddRouter(protocol.MsgIdRoomLeaveReq, &router.RoomRouter{BC: pkg.NewBroadcaster(s)})

	s.Serve()
}

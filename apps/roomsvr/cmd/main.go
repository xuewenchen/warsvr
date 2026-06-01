package main

import (
	"cardwar/apps/roomsvr/internal/router"
	"cardwar/pkg"
	"cardwar/pkg/conf"
	"cardwar/pkg/server"
	"cardwar/protocol"
	"flag"

	"github.com/aceld/zinx/zconf"
)

func main() {
	configPath := flag.String("conf", "config.yml", "path to config file")
	svrID := flag.String("id", "", "RoomSvr ID (matches config services.roomsvr[].id)")
	flag.Parse()

	if err := conf.Load(*configPath); err != nil {
		panic(err)
	}

	cfg := conf.LookupServer(conf.GlobalConfig.Services[conf.SvcRoomSvr], *svrID, conf.SvcRoomSvr)
	host, port := conf.ParseHostPort(cfg.Listen)

	// Dial MatchSvr for room-destroyed notifications
	reg := pkg.NewRegistry(conf.SvcRoomSvr)
	reg.Dial(conf.SvcMatchSvr, nil, pkg.HashRoute)

	s := server.New(&zconf.Config{
		Name:    conf.SvcRoomSvr,
		Host:    host,
		TCPPort: port,
		Mode:    zconf.ServerModeTcp,
	}, conf.SvcRoomSvr, []uint32{
		protocol.MsgIdRoomJoinReq,
		protocol.MsgIdRoomLeaveReq,
	}, []uint32{
		protocol.MsgIdRoomJoinResp,
		protocol.MsgIdRoomLeaveResp,
		protocol.MsgIdRoomEventPush,
	})

	rr := &router.RoomRouter{BC: pkg.NewGateWayBroadcaster(s), Reg: reg}
	s.AddRouter(protocol.MsgIdRoomJoinReq, rr)
	s.AddRouter(protocol.MsgIdRoomLeaveReq, rr)

	s.Serve()
}

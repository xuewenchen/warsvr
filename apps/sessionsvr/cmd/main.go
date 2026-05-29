package main

import (
	"cardwar/apps/sessionsvr/internal/router"
	"cardwar/pkg"
	"cardwar/pkg/conf"
	"cardwar/protocol"
	"flag"

	"github.com/aceld/zinx/zconf"
)

func main() {
	configPath := flag.String("conf", "config.yml", "path to config file")
	svrID := flag.String("id", "", "SessionSvr ID (matches config services.sessionsvr[].id)")
	flag.Parse()

	if err := conf.Load(*configPath); err != nil {
		panic(err)
	}

	cfg := conf.LookupServer(conf.GlobalConfig.Services[conf.SvcSessionSvr], *svrID, conf.SvcSessionSvr)
	host, port := conf.ParseHostPort(cfg.Listen)

	// Registry for connecting to RoomSvr and MatchSvr for TTL cleanup
	reg := pkg.NewRegistry(conf.SvcSessionSvr)
	reg.Dial(conf.SvcRoomSvr, nil, pkg.HashRoute)
	reg.Dial(conf.SvcMatchSvr, nil, pkg.HashRoute)

	s := pkg.NewServer(&zconf.Config{
		Name:    conf.SvcSessionSvr,
		Host:    host,
		TCPPort: port,
		Mode:    zconf.ServerModeTcp,
	})

	sr := &router.SessionRouter{Reg: reg}
	s.AddRouter(protocol.MsgIdSessionSave, sr)
	s.AddRouter(protocol.MsgIdSessionGet, sr)
	s.AddRouter(protocol.MsgIdSessionDisconnect, sr)
	s.AddRouter(protocol.MsgIdSessionReconnect, sr)

	router.StartExpiryScanner(reg)

	s.Serve()
}

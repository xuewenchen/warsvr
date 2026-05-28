package main

import (
	"cardwar/apps/matchsvr/internal/router"
	"cardwar/pkg"
	"cardwar/pkg/conf"
	"cardwar/protocol"
	"flag"

	"github.com/aceld/zinx/zconf"
	"github.com/aceld/zinx/znet"
)

func main() {
	configPath := flag.String("conf", "config.yml", "path to config file")
	svrID := flag.String("id", "", "MatchSvr ID (matches config services.matchsvr[].id)")
	flag.Parse()

	if err := conf.Load(*configPath); err != nil {
		panic(err)
	}

	cfg := conf.LookupServer(conf.GlobalConfig.Services["matchsvr"], *svrID, "MatchSvr")
	host, port := conf.ParseHostPort(cfg.Listen)

	s := znet.NewUserConfServer(&zconf.Config{
		Name:    "MatchSvr",
		Host:    host,
		TCPPort: port,
		Mode:    zconf.ServerModeTcp,
	})

	s.AddRouter(protocol.MsgIdPing, &router.PingRouter{})
	s.AddRouter(protocol.MsgIdGatewayRegister, &router.GatewayRegisterRouter{})
	mr := &router.MatchRouter{BC: pkg.NewBroadcaster(s)}
	s.AddRouter(protocol.MsgIdMatchEnterReq, mr)
	s.AddRouter(protocol.MsgIdMatchAllocateReq, mr)
	s.AddRouter(protocol.MsgIdMatchQueryReq, mr)

	s.Serve()
}

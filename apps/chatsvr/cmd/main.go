package main

import (
	"cardwar/apps/chatsvr/internal/router"
	"cardwar/pkg"
	"cardwar/pkg/conf"
	"cardwar/pkg/server"
	"cardwar/protocol"
	"flag"

	"github.com/aceld/zinx/zconf"
)

func main() {
	configPath := flag.String("conf", "config.yml", "path to config file")
	csID := flag.String("id", "", "ChatSvr ID (matches config services.chatsvr[].id)")
	flag.Parse()

	if err := conf.Load(*configPath); err != nil {
		panic(err)
	}

	csCfg := conf.LookupServer(conf.GlobalConfig.Services[conf.SvcChatSvr], *csID, conf.SvcChatSvr)
	host, port := conf.ParseHostPort(csCfg.Listen)

	cfg := &zconf.Config{
		Name:    conf.SvcChatSvr,
		Host:    host,
		TCPPort: port,
		Mode:    zconf.ServerModeTcp,
	}
	s := server.New(cfg, conf.SvcChatSvr,
		[]uint32{protocol.MsgIdChatReq},  // forward
		[]uint32{protocol.MsgIdChatResp}) // send

	s.AddRouter(protocol.MsgIdChatReq, &router.ChatRouter{BC: pkg.NewGateWayBroadcaster(s)})

	s.Serve()
}

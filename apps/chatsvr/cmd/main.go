package main

import (
	"cardwar/apps/chatsvr/internal/router"
	"cardwar/pkg"
	"cardwar/pkg/conf"
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
	s := pkg.NewServer(cfg)

	s.AddRouter(protocol.MsgIdChatReq, &router.ChatRouter{BC: pkg.NewGateWayBroadcaster(s)})

	s.Serve()
}

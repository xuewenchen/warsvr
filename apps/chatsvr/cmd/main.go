package main

import (
	"cardwar/apps/chatsvr/internal/router"
	"cardwar/common"
	"cardwar/conf"
	"flag"

	"github.com/aceld/zinx/zconf"
	"github.com/aceld/zinx/znet"
)

func main() {
	configPath := flag.String("conf", "config.yml", "path to config file")
	csID := flag.String("id", "", "ChatSvr ID (matches config services.chatsvr[].id)")
	flag.Parse()

	if err := conf.Load(*configPath); err != nil {
		panic(err)
	}

	csCfg := conf.LookupServer(conf.GlobalConfig.Services["chatsvr"], *csID, "ChatSvr")
	host, port := conf.ParseHostPort(csCfg.Listen)

	cfg := &zconf.Config{
		Name:    "ChatSvr",
		Host:    host,
		TCPPort: port,
		Mode:    zconf.ServerModeTcp,
	}
	s := znet.NewUserConfServer(cfg)

	s.AddRouter(common.MsgIdPing, &router.PingRouter{})
	s.AddRouter(common.MsgIdLogin, &router.LoginRouter{})
	s.AddRouter(common.MsgIdChat, &router.ChatRouter{})

	s.Serve()
}

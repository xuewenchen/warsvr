package main

import (
	"cardwar/chatsvr/internal/router"
	"cardwar/common"
	"cardwar/conf"
	"flag"
	"strconv"
	"strings"

	"github.com/aceld/zinx/zconf"
	"github.com/aceld/zinx/znet"
)

func parseHostPort(addr string) (string, int) {
	parts := strings.Split(addr, ":")
	if len(parts) != 2 {
		panic("invalid address: " + addr)
	}
	port, err := strconv.Atoi(parts[1])
	if err != nil {
		panic("invalid port: " + addr)
	}
	return parts[0], port
}

func main() {
	configPath := flag.String("conf", "config.yml", "path to config file")
	csID := flag.String("id", "", "ChatSvr ID (matches config services.chatsvr[].id)")
	flag.Parse()

	if err := conf.Load(*configPath); err != nil {
		panic(err)
	}

	var csCfg conf.ServerNode
	if *csID != "" {
		found := false
		for _, cfg := range conf.GlobalConfig.Services.ChatSvr {
			if cfg.ID == *csID {
				csCfg = cfg
				found = true
				break
			}
		}
		if !found {
			panic("ChatSvr ID not found in config: " + *csID)
		}
	} else {
		csCfg = conf.GlobalConfig.Services.ChatSvr[0]
	}
	host, port := parseHostPort(csCfg.Listen)

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

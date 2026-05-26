package main

import (
	"cardwar/common"
	"cardwar/common/utils"
	"cardwar/conf"
	"cardwar/gateway/internal/router"
	"flag"
	"fmt"

	"github.com/aceld/zinx/znet"
)

func main() {
	// 服务配置
	configPath := flag.String("conf", "config.yml", "path to config file")
	flag.Parse()

	if err := conf.Load(*configPath); err != nil {
		panic(err)
	}

	fmt.Println(utils.ToPrettyJsonForDebug(conf.GlobalConfig))

	s := znet.NewServer()
	s.AddRouter(common.MsgIdPing, &router.PingRouter{})
	s.Serve()
}

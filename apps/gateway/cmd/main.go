package main

import (
	"cardwar/pkg/gateway"
	"flag"
)

func main() {
	configPath := flag.String("conf", "config.yml", "path to config file")
	gwID := flag.String("id", "", "Gateway ID (matches config services.gateway[].id)")
	flag.Parse()

	gw, err := gateway.New(*configPath, *gwID)
	if err != nil {
		panic(err)
	}
	gw.Server.Serve()
}

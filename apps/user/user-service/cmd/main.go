package main

import (
	"flag"
	"log"
	"os"
	"strings"

	"cardwar/apps/user/user-service/internal/handler"
	"cardwar/pkg/grpcserver"
	userpb "cardwar/protocol/pb/user"

	"gopkg.in/yaml.v3"
)

type serviceConfig struct {
	Addr string     `yaml:"addr"`
	Etcd etcdConfig `yaml:"etcd"`
}

type etcdConfig struct {
	Endpoints []string `yaml:"endpoints"`
}

func loadConfig(path string) (*serviceConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := &serviceConfig{Addr: ":50051"}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func main() {
	configPath := flag.String("conf", "apps/user/user-service/config.yml", "config file path")
	flag.Parse()

	cfg, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	if addr := os.Getenv("LISTEN_ADDR"); addr != "" {
		cfg.Addr = addr
	}
	if endpoints := os.Getenv("ETCD_ENDPOINTS"); endpoints != "" {
		cfg.Etcd.Endpoints = strings.Split(endpoints, ",")
	}

	srv, err := grpcserver.New(grpcserver.Config{
		Name:          "user-service",
		Addr:          cfg.Addr,
		EtcdEndpoints: cfg.Etcd.Endpoints,
	})
	if err != nil {
		log.Fatalf("create gRPC server: %v", err)
	}

	userpb.RegisterUserServiceServer(srv.GRPC, handler.New(srv.ID()))

	go srv.WaitForShutdown()

	if err := srv.Serve(); err != nil {
		log.Fatalf("serve: %v", err)
	}
}

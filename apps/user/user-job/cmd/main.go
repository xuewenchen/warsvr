package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cardwar/pkg/grpcclient"
	userpb "cardwar/protocol/pb/user"

	"github.com/nats-io/nats.go"
	"gopkg.in/yaml.v3"
)

type jobConfig struct {
	Etcd   etcdConfig `yaml:"etcd"`
	NATS   string     `yaml:"nats"`
	Target string     `yaml:"target"`
}

type etcdConfig struct {
	Endpoints []string `yaml:"endpoints"`
}

func loadJobConfig(path string) (*jobConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := &jobConfig{NATS: nats.DefaultURL, Target: "localhost:50051"}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func main() {
	configPath := flag.String("conf", "config.yml", "config file path")
	flag.Parse()

	cfg, err := loadJobConfig(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	// ---- gRPC client to user-service ----
	userConn, err := grpcclient.NewConn(grpcclient.Config{
		ServiceName:   "user-service",
		EtcdEndpoints: cfg.Etcd.Endpoints,
		DirectAddr:    cfg.Target,
	})
	if err != nil {
		log.Fatalf("connect user-service: %v", err)
	}
	defer userConn.Close()

	userClient := userpb.NewUserServiceClient(userConn)

	// ---- NATS consumer ----
	nc, err := nats.Connect(cfg.NATS)
	if err != nil {
		log.Fatalf("connect NATS: %v", err)
	}
	defer nc.Close()

	nc.Subscribe("user.create", func(msg *nats.Msg) {
		var req userpb.CreateUserRequest
		if err := json.Unmarshal(msg.Data, &req); err != nil {
			log.Printf("NATS user.create: bad message: %v", err)
			return
		}
		resp, err := userClient.CreateUser(context.TODO(), &req)
		if err != nil {
			log.Printf("NATS user.create: create user failed: %v", err)
			return
		}
		log.Printf("NATS user.create: user %s created", resp.User.Id)
	})

	log.Printf("user-job: NATS consumer ready on %s", cfg.NATS)

	// ---- cron scheduler ----
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	go func() {
		for range ticker.C {
			resp, err := userClient.Ping(context.TODO(), &userpb.PingRequest{})
			if err != nil {
				log.Printf("cron: ping user-service failed: %v", err)
			} else {
				log.Printf("cron: ping user-service OK (%s)", resp.ServerId)
			}
		}
	}()

	log.Println("user-job running (NATS consumer + cron)")

	// ---- wait for shutdown ----
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Println("user-job shutting down...")
	ticker.Stop()
	nc.Drain()
}

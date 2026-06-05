package etcdutil

import (
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// Config holds configuration for connecting to an etcd cluster.
type Config struct {
	Endpoints   []string      // etcd endpoints, e.g. []string{"192.168.1.110:2379"}
	DialTimeout time.Duration // dial timeout, default 5s
	TTL         int64         // lease TTL in seconds, default 10
}

// Client wraps the etcd v3 client so callers don't need to import etcd directly.
type Client struct {
	Raw *clientv3.Client
	TTL int64
}

// NewConfig returns a Config with sensible defaults.
func NewConfig(endpoints []string) Config {
	return Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
		TTL:         10,
	}
}

// NewClient dials the etcd cluster and returns a wrapped client.
func NewClient(cfg Config) (*Client, error) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   cfg.Endpoints,
		DialTimeout: cfg.DialTimeout,
	})
	if err != nil {
		return nil, err
	}
	return &Client{Raw: cli, TTL: cfg.TTL}, nil
}

// Close shuts down the underlying etcd client.
func (c *Client) Close() error {
	return c.Raw.Close()
}

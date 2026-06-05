package grpcclient

import (
	"fmt"

	"cardwar/pkg/etcdutil"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/resolver"
)

// Config holds the configuration for creating a gRPC client connection.
type Config struct {
	ServiceName   string   // target service name for etcd resolution
	EtcdEndpoints []string // etcd cluster for service discovery
	DirectAddr    string   // fallback direct address when no etcd, e.g. "localhost:50051"
}

// NewConn creates a gRPC client connection. When EtcdEndpoints is non-empty, the
// etcd resolver is registered and used for service discovery. Otherwise, it falls
// back to a direct dial to DirectAddr.
func NewConn(cfg Config) (*grpc.ClientConn, error) {
	var target string

	if len(cfg.EtcdEndpoints) > 0 {
		etcdCfg := etcdutil.NewConfig(cfg.EtcdEndpoints)
		cli, err := etcdutil.NewClient(etcdCfg)
		if err != nil {
			return nil, fmt.Errorf("grpcclient: connect etcd: %w", err)
		}
		// The etcd resolver builder already registers itself with a scheme; we need
		// a resolver.Builder that knows about this specific client. RegisterResolver
		// stores the etcd client for use when resolving.
		etcdutil.RegisterResolver(cli)

		target = fmt.Sprintf("%s:///%s", etcdutil.Scheme, cfg.ServiceName)
	} else {
		target = cfg.DirectAddr
	}

	resolver.SetDefaultScheme("dns")

	conn, err := grpc.NewClient(target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`),
	)
	if err != nil {
		return nil, fmt.Errorf("grpcclient: dial %s: %w", target, err)
	}

	return conn, nil
}

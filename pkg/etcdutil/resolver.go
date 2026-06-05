package etcdutil

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc/resolver"
)

const Scheme = "etcd"

// RegisterResolver creates and registers an etcd-based gRPC resolver builder.
// Call this once at program init before dialing with etcd:/// targets.
func RegisterResolver(cli *Client) {
	b := &builder{cli: cli}
	resolver.Register(b)
}

// builder implements resolver.Builder for scheme "etcd".
type builder struct {
	cli *Client
}

func (b *builder) Scheme() string { return Scheme }

func (b *builder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	serviceName := target.URL.Host
	if serviceName == "" {
		serviceName = target.Endpoint()
	}
	if serviceName == "" {
		return nil, fmt.Errorf("etcd resolver: target %q has no host/endpoint", target)
	}

	prefix := fmt.Sprintf("/services/%s/", serviceName)
	r := &etcdResolver{
		cli:    b.cli,
		prefix: prefix,
		cc:     cc,
		done:   make(chan struct{}),
	}
	r.start()
	return r, nil
}

// etcdResolver implements resolver.Resolver.
type etcdResolver struct {
	cli    *Client
	cc     resolver.ClientConn
	prefix string
	done   chan struct{}

	mu     sync.Mutex
	cancel context.CancelFunc
}

func (r *etcdResolver) start() {
	ctx, cancel := context.WithCancel(context.Background())
	r.mu.Lock()
	r.cancel = cancel
	r.mu.Unlock()

	// Initial fetch
	r.update(ctx)

	// Watch for changes
	go r.watch(ctx)
}

func (r *etcdResolver) update(ctx context.Context) {
	resp, err := r.cli.Raw.Get(ctx, r.prefix, clientv3.WithPrefix())
	if err != nil {
		log.Printf("etcd resolver: get prefix %s: %v", r.prefix, err)
		return
	}

	addrs := make([]resolver.Address, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		addr := strings.TrimPrefix(string(kv.Key), r.prefix)
		addrs = append(addrs, resolver.Address{
			Addr:       string(kv.Value),
			ServerName: addr,
		})
	}
	r.cc.UpdateState(resolver.State{Addresses: addrs})
}

func (r *etcdResolver) watch(ctx context.Context) {
	wch := r.cli.Raw.Watch(ctx, r.prefix, clientv3.WithPrefix())
	for {
		select {
		case resp, ok := <-wch:
			if !ok {
				return
			}
			if err := resp.Err(); err != nil {
				log.Printf("etcd resolver: watch error: %v", err)
				continue
			}
			// Re-fetch full list on any change
			r.update(ctx)
		case <-r.done:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (r *etcdResolver) ResolveNow(opts resolver.ResolveNowOptions) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	r.update(ctx)
}

func (r *etcdResolver) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cancel != nil {
		r.cancel()
	}
	close(r.done)
}

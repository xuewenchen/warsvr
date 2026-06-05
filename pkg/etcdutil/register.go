package etcdutil

import (
	"context"
	"fmt"
	"log"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// ServiceKey returns the standard etcd key for a service instance.
// Convention: /services/<service-name>/<instance-id>
func ServiceKey(serviceName, instanceID string) string {
	return fmt.Sprintf("/services/%s/%s", serviceName, instanceID)
}

// Register creates a lease, puts key→value into etcd, and starts an auto-renewal
// keepalive goroutine. Returns a cancel function — the caller MUST call it on
// shutdown to revoke the lease (which auto-deletes the key).
func Register(cli *Client, key, value string) (cancel func(), err error) {
	ctx, clean := context.WithTimeout(context.Background(), time.Duration(cli.TTL)*time.Second)
	defer clean()

	lease, err := cli.Raw.Grant(ctx, cli.TTL)
	if err != nil {
		return nil, fmt.Errorf("etcd grant lease: %w", err)
	}

	_, err = cli.Raw.Put(ctx, key, value, clientv3.WithLease(lease.ID))
	if err != nil {
		return nil, fmt.Errorf("etcd put: %w", err)
	}

	// keepalive in background
	keepCtx, keepCancel := context.WithCancel(context.Background())
	keepCh, keepErr := cli.Raw.KeepAlive(keepCtx, lease.ID)
	if keepErr != nil {
		keepCancel()
		return nil, fmt.Errorf("etcd keepalive: %w", keepErr)
	}

	go func() {
		for range keepCh {
			// drain keepalive responses
		}
	}()

	cancel = func() {
		keepCancel()
		revokeCtx, revokeClean := context.WithTimeout(context.Background(), 3*time.Second)
		defer revokeClean()
		if _, err := cli.Raw.Revoke(revokeCtx, lease.ID); err != nil {
			log.Printf("etcd revoke lease: %v", err)
		}
	}

	log.Printf("etcd: registered %s → %s (lease=%d, ttl=%ds)", key, value, lease.ID, cli.TTL)
	return cancel, nil
}

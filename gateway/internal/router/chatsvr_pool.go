package router

import (
	"hash/fnv"

	"cardwar/conf"

	"github.com/aceld/zinx/ziface"
)

var _ BackendPool = (*ChatSvrPool)(nil)

type ChatSvrPool struct {
	*BaseBackendPool
}

func NewChatSvrPool(conns []ziface.IConnection, servers []conf.ServerNode, routers []BackendRouterConfig) *ChatSvrPool {
	return &ChatSvrPool{
		BaseBackendPool: newBaseBackendPool(conns, servers, routers),
	}
}

func (p *ChatSvrPool) Route(playerID string) ziface.IConnection {
	healthy := p.HealthyConns()
	if len(healthy) == 0 {
		return nil
	}
	h := fnv.New32a()
	h.Write([]byte(playerID))
	idx := int(h.Sum32()) % len(healthy)
	return healthy[idx]
}

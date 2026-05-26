package router

import (
	"hash/fnv"

	"github.com/aceld/zinx/ziface"
)

var _ BackendPool = (*ChatSvrPool)(nil)

type ChatSvrPool struct {
	conns []ziface.IConnection
}

func NewChatSvrPool(conns []ziface.IConnection) *ChatSvrPool {
	return &ChatSvrPool{conns: conns}
}

func (p *ChatSvrPool) Route(playerID string) ziface.IConnection {
	h := fnv.New32a()
	h.Write([]byte(playerID))
	idx := int(h.Sum32()) % len(p.conns)
	return p.conns[idx]
}

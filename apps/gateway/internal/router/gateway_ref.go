package router

import (
	"cardwar/common"
	"sync"

	"github.com/aceld/zinx/ziface"
)

// GatewayRef holds Gateway-specific state. Embeds Registry for backend connection management.
type GatewayRef struct {
	*common.Registry
	Server      ziface.IServer
	PlayerConns *sync.Map // playerID → connID (uint64)
}

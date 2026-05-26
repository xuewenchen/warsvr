package router

import (
	"cardwar/pkg"
	"sync"

	"github.com/aceld/zinx/ziface"
)

// GatewayRef holds Gateway-specific state. Embeds Registry for backend connection management.
type GatewayRef struct {
	*pkg.Registry
	Server      ziface.IServer
	PlayerConns *sync.Map // playerID → connID (uint64)
}

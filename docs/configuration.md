# Configuration

`config.yml` — loaded by `pkg/conf/config.go` into `conf.GlobalConfig`.

## Structure

```yaml
services:                    # ServicesConfig — map of service name → []ServerNode
  gateway: [...]             # client-facing servers
  chatsvr: [...]             # backend servers
  matchsvr: [...]
  roomsvr: [...]

gateway:                     # GatewayConfig
  jwt_secret: "..."          # HMAC-SHA256 secret for JWT validation
  routes:                    # map of backend → BackendRoute
    <backend>:
      forward: [...]         # client→backend msgIDs
      route_key: <key>       # "connId", "playerId", "server_id", or any conn property
      route_type: <type>     # "hash" (default), "random", "direct"
```

## ServerNode

```go
type ServerNode struct {
    ID         string `yaml:"id"`          // instance identifier (cs-1, gw-1, etc.)
    TCPListen  string `yaml:"tcp_listen"`  // gateway TCP port
    WSListen   string `yaml:"ws_listen"`   // gateway WebSocket port
    Listen     string `yaml:"listen"`      // backend TCP port
    PublicAddr string `yaml:"public_addr"` // advertised address
}
```

## GatewayConfig

```go
type GatewayConfig struct {
    JWTSecret string                `yaml:"jwt_secret"`
    Routes    map[string]BackendRoute `yaml:"routes"`
}

type BackendRoute struct {
    Forward   []uint32 `yaml:"forward"`     // msgIDs to forward to this backend
    RouteKey  string   `yaml:"route_key"`   // connection property to use as routing key
    RouteType string   `yaml:"route_type"`  // "hash" | "random" | "direct"
}
```

## Route Types

| type | behavior | when to use |
|---|---|---|
| `hash` | `FNV32(key) % len(healthy)` — consistent per key | Stateless services (chatsvr, matchsvr) |
| `random` | `rand.Intn(len(healthy))` — no affinity | Stateless, no session needed |
| `direct` | Iterate healthy, find `conn.GetProperty("server_id") == key` | Stateful (roomsvr): client/upstream sets which instance |

## Full Example

```yaml
services:
  gateway:
    - id: gw-1
      tcp_listen: 0.0.0.0:9000
      ws_listen: 0.0.0.0:9001
      public_addr: 127.0.0.1:9000
    - id: gw-2
      tcp_listen: 0.0.0.0:9002
      ws_listen: 0.0.0.0:9003
      public_addr: 127.0.0.1:9002

  chatsvr:
    - id: cs-1
      listen: 0.0.0.0:8001
      public_addr: 127.0.0.1:8001

  matchsvr:
    - id: matchsvr-1
      listen: 0.0.0.0:8004
      public_addr: 127.0.0.1:8004

  roomsvr:
    - id: roomsvr-1
      listen: 0.0.0.0:8005
      public_addr: 127.0.0.1:8005

gateway:
  jwt_secret: "change-me-in-production"
  routes:
    chatsvr:
      forward: [5]
      route_key: playerId
      route_type: hash
    matchsvr:
      forward: [11, 18, 20]
      route_key: connId
      route_type: hash
    roomsvr:
      forward: [14, 16]
      route_key: server_id
      route_type: direct
```

## Multi-Instance

Add more instances to scale:

```yaml
chatsvr:
  - id: cs-1
    listen: 0.0.0.0:8001
  - id: cs-2
    listen: 0.0.0.0:8002
  - id: cs-3
    listen: 0.0.0.0:8003
```

Gateway auto-discovers new instances on hot-reload via `Pool.Sync()`. No restart needed.

# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Architecture

This is a game/chat server (`cardwar`) built on **Zinx v1.2.8** — a TCP framework that provides message routing based on message IDs. The module path is `cardwar`.

### Service topology

```
Client (WebSocket + JWT) ───> Gateway ───> ChatSvr (TCP)
                       <───         <───
```

- **Gateway** (`apps/gateway/cmd/main.go`): Dual-protocol server (TCP:8999 + WebSocket:9000) facing clients. **JWT authentication at connection time** via `SetWebsocketAuth` — validates token before WebSocket upgrade, rejects invalid connections with HTTP 401. **Pure forwarding layer**: config-driven routing with two generic routers (ForwardRouter, ResponseRouter) — no per-message-type routers. Maintains internal TCP connections to backend services via `pkg.Registry`.
- **ChatSvr** (`apps/chatsvr/cmd/main.go`): A single TCP server (:8001) handling business logic — chat message processing and broadcast generation. No longer handles login; player identity is verified by Gateway before the connection reaches ChatSvr.
- **Test Client** (`tools/testclient/cmd/main.go`): WebSocket test client that reads `config.yml` for JWT secret, auto-generates a JWT token for the given player ID, connects to Gateway with `ws://127.0.0.1:9000/ws?token=<JWT>`, then sends chat messages on a 5-second loop.

### Authentication flow (JWT)

1. Client obtains a JWT token signed with `HS256` containing `{"playerId": "xxx"}` from an external HTTP service.
2. Client connects to Gateway WebSocket with `ws://host:9000/ws?token=<JWT>`.
3. Gateway's `SetWebsocketAuth` callback validates the JWT **before** the WebSocket upgrade. If invalid, Gateway returns HTTP 401 and the connection is rejected.
4. On success, the `playerId` is extracted from JWT claims, stored in a `pendingAuths` map (keyed by `RemoteAddr`), then picked up by `OnConnStart` to set `conn.SetProperty("playerId", ...)` and `PlayerConns` mapping.
5. No Login/LoginRsp messages are forwarded between Gateway and ChatSvr.

### Message flow (protobuf definitions in `protocol/proto/cardwar.proto`, msgIDs in `protocol/msgid.go`, generated code in `protocol/pb/`)

| MsgID | Name       | Direction                          |
|-------|------------|------------------------------------|
| 1     | Ping       | Client → Gateway (local pong)      |
| 2     | Pong       | Gateway → Client                   |
| 3     | Login      | _deprecated — JWT auth replaces this_ |
| 4     | Chat       | Client → Gateway → ChatSvr         |
| 5     | LoginRsp   | _deprecated — JWT auth replaces this_ |
| 6     | Broadcast  | ChatSvr → Gateway → All Clients    |

### Key types

- **`pb.Envelope`** (`protocol/proto/cardwar.proto`): Internal wrapper between Gateway and backends. Contains `conn_id` (client session), `data` (raw protobuf bytes), and `conn_tags` (metadata the backend wants Gateway to set on the client connection, e.g. `{"playerId": "p1"}`). `conn_id=0` means broadcast.
- **`pb.LoginReq`**, **`pb.LoginRsp`**: Deprecated — replaced by JWT auth at Gateway connection level.
- **`pb.ChatReq`**, **`pb.BroadcastPush`**: Client-facing protocol messages defined in protobuf.
- **`router.GatewayRef`** (`apps/gateway/internal/router/gateway_ref.go`): Shared state on Gateway — embeds `*pkg.Registry` for backend connection management (Dial/RouteTo), holds `Server` (the client-facing IServer), `PlayerConns` (`sync.Map` of playerID → connID), and the forward route index for msgID→backend lookup. Thread-safe route table via `sync.RWMutex`.

### Gateway routing (config-driven, generic)

Gateway uses **two generic routers** instead of per-message-type routers:

- **`ForwardRouter`** (`apps/gateway/internal/router/forward_router.go`): Registered for msgIDs 1–1000 on the WebSocket server. On each message, looks up `config.yml`'s `gateway.routes` to find which backend handles this msgID, reads the routing key from connection metadata (`connId` or `playerId`), wraps raw bytes in `pb.Envelope{ConnId, Data}`, and forwards via `RouteTo(backend, key)`. Falls back to `connId` when `playerId` is not yet set.
- **`ResponseRouter`** (`apps/gateway/internal/router/response_router.go`): Registered on each backend TCP connection for the response msgIDs listed in config. Unwraps `pb.Envelope`, applies `conn_tags` to the client connection's properties (and updates `PlayerConns` if `playerId` tag present), then forwards `env.Data` to the client. If `ConnId == 0`, broadcasts to all connected clients.

Routes are defined in `config.yml`:

```yaml
gateway:
  jwt_secret: "change-me-in-production"
  routes:
    chatsvr:
      forward: [4]          # client→backend msgIDs
      response: [6]         # backend→client msgIDs
      route_key: playerId   # "connId" or "playerId"
```

**Adding a new backend** (e.g. RoomSvr):
```yaml
gateway:
  routes:
    chatsvr:
      forward: [4]
      response: [6]
      route_key: playerId
    roomsvr:                # new backend
      forward: [7, 8, 9]
      response: [10, 11]
      route_key: connId
```
Plus add `roomsvr` to `services` section. No Go code changes needed.

**Adding new msgIDs to an existing backend**: just edit the `forward`/`response` lists in `config.yml`. Gateway hot-reloads routes without restart. New msgIDs must be within 1–1000 (the pre-registered range).

### Config hot-reload

`conf.Watch(path, callback)` (`conf/watcher.go`) uses `fsnotify` to watch the config file. On change, it reloads `GlobalConfig` and invokes the callback. Any service can use it:

```go
conf.Watch(configPath, func(cfg *conf.Config) {
    gw.SetRoutes(router.BuildRouteIndex(cfg.Gateway))
})
```

### Backend abstraction (`pkg/pool.go`, `pkg/registry.go`)

**`pkg.Registry`**: Manages connections to multiple backend services:
```go
reg := pkg.NewRegistry()
reg.Dial("chatsvr", routers, pkg.HashRoute)
conn := reg.RouteTo("chatsvr", key)
```

For services with extra state (e.g., Gateway), embed `*pkg.Registry` in a wrapper struct. `Dial` and `RouteTo` are promoted automatically.

**`pkg.Dial(service, routers, routeFn) *Pool`**: Connects to all configured instances of a backend and returns a Pool. Reads config, creates TCP clients, wires reconnection callbacks, blocks until all connections are established.

**`pkg.Pool`**: Generic connection pool with:
- Thread-safe connection management with `HealthyConns()`
- Automatic reconnection with exponential backoff (1s → 2s → … → 30s)
- Pluggable routing via `RouteFunc` (`HashRoute`, `RandomRoute`, or custom)
- 15s connection timeout (cancels via `client.Stop()`)

**`pkg.BackendPool`**: Interface with `Route(key string) ziface.IConnection`. `Pool` implements this.

### JWT utilities (`pkg/auth/jwt.go`)

Shared package for JWT generation and validation using HS256:

```go
token, _ := auth.GenerateJWT("player1", secret)
playerID, err := auth.ValidateJWT(token, secret)
```

### Configuration

`config.yml` — loaded by `conf/config.go` into `conf.GlobalConfig`. Structure:

```yaml
services:           # ServicesConfig — map of backend name → []ServerNode
  gateway: [...]
  chatsvr: [...]
gateway:            # GatewayConfig
  jwt_secret: "..." # HMAC-SHA256 secret for JWT validation
  routes:           # map of backend → BackendRoute
    chatsvr:
      forward: [4]
      response: [6]
      route_key: playerId
```

`conf.LookupServer(servers, id, name)` selects a server by ID or falls back to the first entry. Gateway uses `Mode: "tcp,ws"` to start both TCP and WebSocket listeners.


## Commands

```bash
# Terminal 1: Start ChatSvr
go run ./apps/chatsvr/cmd/main.go -conf config.yml
go run ./apps/chatsvr/cmd/main.go -conf config.yml -id chatsvr-1  # specify instance by ID

# Terminal 2: Start Gateway
go run ./apps/gateway/cmd/main.go -conf config.yml
go run ./apps/gateway/cmd/main.go -conf config.yml -id gateway-1  # specify instance by ID

# Terminal 3: Start test clients (WebSocket, multiple instances, different player IDs)
go run ./tools/testclient/cmd/main.go player1
go run ./tools/testclient/cmd/main.go player2
```

Both ChatSvr and Gateway support an optional `-id` flag to select which config entry to use. If omitted, the first entry in the config array is used.


## Key files reference

| File | Purpose |
|---|---|
| `apps/gateway/cmd/main.go` | Gateway entrypoint: JWT auth, backend Dial, route setup |
| `apps/gateway/internal/router/gateway_ref.go` | GatewayRef, BackendRouteInfo, BuildRouteIndex |
| `apps/gateway/internal/router/forward_router.go` | Generic client→backend forwarding |
| `apps/gateway/internal/router/response_router.go` | Generic backend→client response handling |
| `apps/chatsvr/cmd/main.go` | ChatSvr entrypoint |
| `apps/chatsvr/internal/router/chat_router.go` | Chat message processing and broadcast |
| `pkg/pool.go` | Backend connection pool with reconnection |
| `pkg/registry.go` | Multi-backend connection registry |
| `pkg/auth/jwt.go` | JWT generation and validation |
| `conf/config.go` | Config types and loading |
| `conf/watcher.go` | Config file hot-reload via fsnotify |
| `protocol/proto/cardwar.proto` | Protobuf definitions (Envelope, ChatReq, etc.) |
| `protocol/msgid.go` | Flat message ID constants |
| `config.yml` | Service instances, JWT secret, gateway routes |

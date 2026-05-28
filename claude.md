# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Rules

- **gofmt**: After writing or editing any `.go` file, run `gofmt -w <file>` before declaring the work complete.

## Architecture

This is a game/chat server (`cardwar`) built on **Zinx v1.2.8** — a TCP framework that provides message routing based on message IDs. The module path is `cardwar`.

### Service topology

```
Client A ───> Gateway-1 ──┐
Client B ───> Gateway-2 ──┼──> ChatSvr (TCP server)
Client C ───> Gateway-1 ──┘
```

Multiple Gateway instances can connect to a single ChatSvr. Each Gateway `Dial`s ChatSvr, creating a TCP client connection. From ChatSvr's perspective, each Gateway is an incoming connection in `Server.GetConnMgr()`.

- **Gateway** (`apps/gateway/cmd/main.go`): Dual-protocol server (TCP:8999 + WebSocket:9000) facing clients. **JWT authentication at connection time** via `SetWebsocketAuth` — validates token before WebSocket upgrade, rejects invalid connections with HTTP 401. **Pure forwarding layer**: config-driven routing with two generic routers (ForwardRouter, ResponseRouter) — no per-message-type routers. Gateway is **stateless with respect to other Gateways**; it never communicates with peer Gateways. Maintains internal TCP connections to backend services via `pkg.Registry`.
- **ChatSvr** (`apps/chatsvr/cmd/main.go`): A single TCP server handling business logic. Maintains `PlayerConns` (`sync.Map` of playerId → Gateway connection) learned passively from incoming Envelope `conn_tags["player_id"]`. Uses this for cross-Gateway private message routing. For global broadcast, iterates ALL Gateway connections.
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
| 6     | ChatPush   | ChatSvr/any backend → Gateway → Client(s) |

### Chat protocol

Chat supports **global** and **private** messaging through a single request/response pair.

```
Client A ── ChatReq{content, target_player_id=""} ──> ChatSvr ──> ChatPush (global) ──> all clients
Client A ── ChatReq{content, target_player_id="B"} ──> ChatSvr ──> ChatPush (private) ──> Client B only
Server   ── ChatPush ──> all clients (system broadcast)
```

**ChatReq** (msgID=4, client→server): `{content, target_player_id}` — `target_player_id` empty = global, non-empty = private recipient. The sender's `player_id` is NOT in the message body; ForwardRouter auto-injects it into Envelope `conn_tags["player_id"]`.

**ChatPush** (msgID=6, server→client): `{sender_player_id, content, timestamp, target_player_id}` — `target_player_id` lets the client distinguish global messages from private messages directed at them.

**Private routing**: ChatSvr sets `conn_tags["target_player_id"] = "B"` in the Envelope. Gateway's ResponseRouter looks up `PlayerConns` and delivers to that specific player. Global messages use `conn_id=0` (broadcast) sent to ALL Gateway connections.

**Multi-Gateway**: ChatSvr passively learns `playerId → Gateway connection` from every incoming Envelope's `conn_tags["player_id"]`. Global broadcast iterates `Server.GetConnMgr().Range()` to send to all Gateways. Private messages are routed to the target player's Gateway connection. Gateway itself is unaware of other Gateways — all cross-Gateway coordination happens at the backend (ChatSvr) level.

### Key types

- **`pb.Envelope`** (`protocol/proto/cardwar.proto`): Internal wrapper between Gateway and backends. Contains `conn_id` (client session, 0=broadcast), `data` (raw protobuf bytes), and `conn_tags` (key-value metadata). `conn_tags["player_id"]` is auto-set by ForwardRouter for sender identity; `conn_tags["target_player_id"]` triggers private routing in ResponseRouter.
- **`pb.LoginReq`**, **`pb.LoginRsp`**: Deprecated — replaced by JWT auth at Gateway connection level.
- **`pb.ChatReq`**, **`pb.ChatPush`**: Client-facing protobuf messages. `ChatReq{content, target_player_id}` for input; `ChatPush{sender_player_id, content, timestamp, target_player_id}` for output.
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
      route_key: playerId   # "connId", "playerId", or custom property
      route_type: hash      # "hash" (default) or "random"
```

**Adding a new backend** (e.g. RoomSvr):
```yaml
gateway:
  routes:
    chatsvr:
      forward: [4]
      route_key: playerId
    roomsvr:
      forward: [7, 8, 9]
      route_key: roomId     # custom property for stateful routing
      route_type: hash
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

**`pkg.Dial(service, routers, routeFn, registerMsgID) *Pool`**: Connects to all configured instances of a backend. Waits up to 3s for connections, then proceeds regardless — failed instances auto-reconnect in background.

**`pkg.Pool`**: Generic connection pool with:
- Thread-safe connection management with `HealthyConns()`
- Automatic reconnection with exponential backoff (200ms → … → 5s), rate-limited logging
- Pluggable routing via `RouteFunc` (`HashRoute`, `RandomRoute`, or custom). Use `pkg.RouteFuncFor(type)` to select by config.
- `Sync()` for hot-reload: adds new servers, removes deleted ones atomically

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
      route_key: playerId
      route_type: hash   # "hash" (default) or "random"
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
go run ./tools/testclient/cmd/main.go 1001
go run ./tools/testclient/cmd/main.go 1002
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

### Wire format (Zinx DataPack)

Zinx v1.2.8 uses **BigEndian `DataPack`** by default. Every TCP/WebSocket message is framed:

```
[4B msgID BigEndian][4B dataLen BigEndian][protobuf body]
```

Example — sending ChatReq (msgID=5, "hello"):
```
00 00 00 05   00 00 00 15   0A 0D 68 65 6C 6C 6F ...
   msgID=5      dataLen=21     protobuf ChatReq bytes
```

Two implementations exist in `zpack/`:
- **`DataPack`** (default): BigEndian, msgID first — used by all servers/clients
- **`DataPackLtv`** (legacy): LittleEndian, dataLen first — backward compat only

Non-Go clients MUST use BigEndian msgID-first format when implementing the wire protocol.

# Architecture

## Service Topology

```
Client A ───> Gateway-1 ──┐
Client B ───> Gateway-2 ──┼──> ChatSvr (TCP)
Client C ───> Gateway-1 ──┤──> MatchSvr (TCP)
                            ├──> RoomSvr (TCP)
                            └──> SessionSvr (TCP)
```

Multiple Gateway instances connect to each backend. Each Gateway `Dial`s every backend, creating TCP client connections. From the backend's perspective, each Gateway is an incoming connection in `Server.GetConnMgr()`.

- **Gateway** (`apps/gateway/cmd/main.go`): Dual-protocol server (TCP:9000 + WebSocket:9001) facing clients. JWT auth at connection time. Pure forwarding layer — ForwardRouter + ResponseRouter dynamically registered via ServiceHello. Stateless with respect to other Gateways.
- **ChatSvr** (`apps/chatsvr/cmd/main.go`): Chat processing. Maintains no player state; player identity from Envelope `conn_tags["player_id"]`. Global broadcast iterates all Gateway connections via `pkg.Broadcaster`.
- **MatchSvr** (`apps/matchsvr/cmd/main.go`): Matchmaking pool + roomsvr directory. `MatchAllocateReq` assigns a roomsvr, `MatchQueryReq` looks up existing match, `MatchEnterReq` enters competitive queue.
- **RoomSvr** (`apps/roomsvr/cmd/main.go`): Room lifecycle. `RoomJoinReq` auto-creates room on first join; `RoomLeaveReq` removes player, auto-destroys when empty.

## Message Flow

### Chat

```
Client A ── ChatReq{content, target_player_id=0} ──> ChatSvr ──> ChatResp (global) ──> all clients
Client A ── ChatReq{content, target_player_id=B} ──> ChatSvr ──> ChatResp (private) ──> Client B only
Server   ── ChatResp ──> all clients (system broadcast)
```

**ChatReq** (msgID=5): `{content, target_player_id}` — 0=global, non-zero=private. Sender's `player_id` is NOT in the body; ForwardRouter auto-injects it into Envelope `conn_tags["player_id"]`.

**ChatResp** (msgID=6): `{sender_player_id, content, timestamp, target_player_id}`.

### Multi-Gateway

ChatSvr passively learns `playerId → Gateway connection` from incoming Envelope `conn_tags["player_id"]`. Global broadcast iterates `Server.GetConnMgr().Range()` sending to all Gateways. Private messages go to all Gateways with `conn_tags["target_player_id"]`; each Gateway's ResponseRouter checks its own `PlayerConns` and ignores if not local.

## Gateway Routing

Gateway uses two generic routers, dynamically registered via ServiceHello:

- **ForwardRouter**: Looks up per-msgID route table (populated by ServiceHello), resolves route key from conn properties (`playerId`, `connId`, `room_server_id`, etc.), wraps in `Envelope{ConnId, Data, ConnTags}`, forwards to backend via `RouteTo(backend, key)`.
- **ResponseRouter**: Unwraps `Envelope`, applies `conn_tags` to client connection properties, forwards `env.Data` to client. `conn_id=0` = broadcast. `conn_tags["target_player_id"]` = private delivery.

### ServiceHello — Dynamic Route Registration

On connect, the Gateway sends `SERVICE_IDENTITY` (1001) to each backend. The backend replies with `SERVICE_HELLO` (1009) listing:

- `msg_ids` — msgIDs the backend handles (forward). Gateway registers ForwardRouter for each on its WebSocket server.
- `send_msg_ids` — msgIDs the backend may send (response/push). Gateway registers ResponseRouter for each on that backend's TCP connection.

This eliminates the need to manually list `forward:` msgIDs in config.yml — the forward list is now optional and deprecated.

Route types:

| type | behavior | use case |
|---|---|---|
| `hash` | `FNV32(key) % len(healthy)` — consistent per key | Stateless services (chatsvr, matchsvr) |
| `random` | Pick any healthy connection randomly | Stateless, no affinity |
| `direct` | Match `conn.GetProperty("server_id")` == route key | Stateful (roomsvr): MatchSvr sets `room_server_id` on client conn, ForwardRouter passes it as key, DirectRoute matches backend conn |

### Config hot-reload

`conf.Watch(path, callback)` uses `fsnotify`. On config change, reloads `GlobalConfig` and calls the callback. Gateway's callback updates routes AND syncs backend connections via `Pool.Sync()`.

## Backend Abstraction

### `pkg.Pool`

Manages backend connections with:
- Thread-safe connections via `HealthyConns()`
- Auto-reconnection with exponential backoff (200ms → … → 5s), rate-limited logging
- Pluggable routing via `RouteFunc` (`HashRoute`, `RandomRoute`, `DirectRoute`)
- `Pool.Sync()` for hot-reload: adds new servers, removes deleted ones atomically
- `Dial` waits up to 3s, then proceeds with partial connections
- `AddConnectionRouter()` for dynamic router registration on existing connections

### `pkg.Broadcaster`

Sends messages to all connected Gateways. Filters by `conn_type="gateway"` property (set via `SERVICE_IDENTITY` msgID 1001 on connect). Methods: `ToAll`, `ToPlayer`, `ToConn`.

### `pkg.Registry`

Multi-backend connection manager:
```go
reg := pkg.NewRegistry("gateway")
reg.Dial("chatsvr", routers, pkg.HashRoute)
reg.SyncBackend("chatsvr", routers, pkg.HashRoute) // hot-reload
reg.Pool("chatsvr") // access BackendPool for dynamic router registration
conn := reg.RouteTo("chatsvr", key)
```

## Server Constructors

- **Backend services**: `server.New(cfg, service, forwardMsgIDs, sendMsgIDs)` — `pkg/server/server.go`
- **Gateway**: `gateway.New(configPath, gwID)` — `pkg/gateway/gateway.go`

Both are reusable; new gateway projects just import `cardwar/pkg/gateway`.

## Message IDs

Defined in `protocol/proto/msgid.proto`, Go aliases in `protocol/msgid.go`.

| MsgID | Name | Direction |
|---|---|---|
| 1 | Ping | Client → Gateway (local pong) |
| 2 | Pong | Gateway → Client |
| 5 | ChatReq | Client → Gateway → ChatSvr |
| 6 | ChatResp | ChatSvr → Gateway → Client(s) |
| 11 | MatchEnterReq | Client → Gateway → MatchSvr (queue pool) |
| 12 | MatchEnterResp | MatchSvr → Gateway → Client |
| 13 | MatchResultPush | MatchSvr → Gateway → Client(s) |
| 14 | RoomJoinReq | Client → Gateway → RoomSvr |
| 15 | RoomJoinResp | RoomSvr → Gateway → Client |
| 16 | RoomLeaveReq | Client → Gateway → RoomSvr |
| 17 | RoomLeaveResp | RoomSvr → Gateway → Client |
| 18 | MatchAllocateReq | Client → Gateway → MatchSvr (assign roomsvr) |
| 19 | MatchAllocateResp | MatchSvr → Gateway → Client |
| 20 | MatchQueryReq | Client → Gateway → MatchSvr (lookup match) |
| 21 | MatchQueryResp | MatchSvr → Gateway → Client |
| 22 | RoomDestroyedPush | RoomSvr → MatchSvr (internal) |
| 23 | RoomEventPush | RoomSvr → Gateway → Client(s) |
| 1001 | ServiceIdentity | Caller → Backend (on connect) |
| 1002 | SessionSave | Gateway → SessionSvr (sync conn_tags) |
| 1003 | SessionGet | Gateway → SessionSvr (query session) |
| 1004 | SessionDisconnect | Gateway → SessionSvr (mark disconnected) |
| 1005 | SessionReconnect | Gateway → SessionSvr (mark reconnected) |
| 1006 | SessionForceLeave | SessionSvr → RoomSvr (TTL expired) |
| 1007 | SessionForceLeaveQueue | SessionSvr → MatchSvr (TTL expired) |
| 1008 | SessionReconnected | Gateway → RoomSvr (update conn ref) |
| 1009 | ServiceHello | Backend → Caller (announce msgIDs) |

### Session Reconnection

On disconnect, Gateway marks the player's session in SessionSvr (120s TTL)
without deleting state. On reconnect within TTL, Gateway restores connection
properties (room_server_id, match_id) and notifies RoomSvr to update stale conn
references. If TTL expires, SessionSvr sends cleanup messages to RoomSvr
(force leave room) and MatchSvr (force leave queue).

Session struct fields are protected by `sync.RWMutex` to prevent data races
between concurrent handlers and the TTL expiry scanner.

## Key Types

- **`pb.Envelope`**: Internal wrapper. `conn_id` (0=broadcast), `data`, `conn_tags` (metadata). `conn_tags["player_id"]` auto-injected by ForwardRouter. `conn_tags["target_player_id"]` triggers private routing. `conn_tags["room_server_id"]` set by MatchSvr for DirectRoute to roomsvr.
- **`pb.ChatReq`/`pb.ChatResp`**: Chat protocol messages.
- **`pb.MatchEnterReq`/`pb.MatchAllocateReq`/`pb.MatchQueryReq`**: Match protocol messages.
- **`pb.RoomJoinReq`/`pb.RoomLeaveReq`**: Room protocol messages.
- **`pb.ServiceHello`**: Backend capability announcement. `service`, `msg_ids` (forward), `send_msg_ids` (response/push).

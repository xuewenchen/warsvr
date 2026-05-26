# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Architecture

This is a game/chat server (`cardwar`) built on **Zinx v1.2.8** — a TCP framework that provides message routing based on message IDs. The module path is `cardwar`.

### Service topology

```
Client (WebSocket) ───> Gateway ───> ChatSvr (TCP)
                <───         <───
```

- **Gateway** (`apps/gateway/cmd/main.go`): Dual-protocol server (TCP:8999 + WebSocket:9000) facing clients. Maintains internal TCP connections to backend services via `common.Registry`. Wraps incoming client messages in an `Envelope` (routing metadata with `ConnID`) and forwards them to ChatSvr. Routes responses back to the correct client connection, or broadcasts to all clients when `ConnID == 0`.
- **ChatSvr** (`apps/chatsvr/cmd/main.go`): A single TCP server (:8001) handling business logic — login validation, chat message processing, and broadcast generation. Stores logged-in player state in memory (`sync.Map`).
- **Test Client** (`tools/testclient/cmd/main.go`): WebSocket test client that connects to Gateway (ws://127.0.0.1:9000/ws), logs in with the given player ID, then sends chat messages on a 5-second loop.

### Message flow (protocol in `common/chat_proto.go`)

| MsgID | Name       | Direction                          |
|-------|------------|------------------------------------|
| 1     | Ping       | Client → Gateway / Client → ChatSvr |
| 2     | Pong       | Gateway → Client / ChatSvr → Client |
| 3     | Login      | Client → Gateway → ChatSvr         |
| 4     | Chat       | Client → Gateway → ChatSvr         |
| 5     | LoginRsp   | ChatSvr → Gateway → Client         |
| 6     | Broadcast  | ChatSvr → Gateway → All Clients    |

### Key types

- **`common.Envelope`**: Internal wrapper between Gateway and ChatSvr. Contains `ConnID` (client session) and `Data` (original message as raw JSON). `ConnID=0` means broadcast.
- **`common.LoginMsg`**, **`LoginRspMsg`**, **`ChatMsg`**, **`BroadcastMsg`**: Client-facing protocol messages, all JSON-encoded.
- **`router.GatewayRef`** (`apps/gateway/internal/router/gateway_ref.go`): Shared state on Gateway — embeds `*common.Registry` for backend connection management (Dial/RouteTo), adds `Server` (the client-facing IServer) and `PlayerConns` (`sync.Map` of playerID → connID).

### Routing pattern

Each message type gets a router struct embedding `znet.BaseRouter` and implementing `Handle(request ziface.IRequest)`. Routers are registered on the server with `s.AddRouter(msgID, &router.XxxRouter{})`. Gateway routers that need to forward to ChatSvr or access Gateway state hold a `*GatewayRef` field injected at construction.

### Login flow

1. Client sends `LoginMsg{PlayerID}` to Gateway (msgId=3).
2. Gateway wraps it in `Envelope{ConnID, Data}` and forwards to ChatSvr.
3. ChatSvr validates, stores the player in `loggedInPlayers` sync.Map, returns `LoginRspMsg{Success: true}` in an `Envelope` (msgId=5).
4. Gateway's `LoginRspRouter` receives it, maps `playerID → connID` in `PlayerConns`, sets `playerId` property on the client connection, and forwards the response to the client.

### Chat/broadcast flow

1. Client sends `ChatMsg{PlayerID, Content}` to Gateway (msgId=4).
2. Gateway forwards to ChatSvr in an `Envelope`.
3. ChatSvr checks login state, constructs a `BroadcastMsg{PlayerID, Content, Timestamp}`, wraps in `Envelope{ConnID: 0, Data}` and sends back (msgId=6).
4. Gateway's `BroadcastRouter` sees `ConnID=0`, iterates all client connections and sends the broadcast message to every connected client.

### Backend abstraction (`common/pool.go`, `common/registry.go`)

**`common.Registry`**: Manages connections to multiple backend services. Any service can use it:
```go
reg := common.NewRegistry()
reg.Dial("chatsvr", routers, common.HashRoute)
conn := reg.RouteTo("chatsvr", key)
```

For services with extra state (e.g., Gateway), embed `*common.Registry` in a wrapper struct. `Dial` and `RouteTo` are promoted automatically.

**`common.Dial(service, routers, routeFn) *Pool`**: Connects to all configured instances of a backend and returns a Pool. Reads config, creates TCP clients, wires reconnection callbacks, blocks until all connections are established.

**`common.Pool`**: Generic connection pool with:
- Thread-safe connection management with `HealthyConns()`
- Automatic reconnection with exponential backoff (1s → 2s → … → 30s)
- Pluggable routing via `RouteFunc` (`HashRoute`, `RandomRoute`, or custom)
- 15s connection timeout (cancels via `client.Stop()`) — not dependent on Zinx retry

**`common.BackendPool`**: Interface with `Route(key string) ziface.IConnection`. `Pool` implements this.

### Configuration

`config.yml` — loaded by `conf/config.go` into `conf.GlobalConfig`. `ServicesConfig` is `map[string][]ServerNode` keyed by backend name (`"gateway"`, `"chatsvr"`, etc.). `conf.LookupServer(servers, id, name)` selects a server by ID or falls back to the first entry.


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
# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Terminal 1: Start ChatSvr
go run chatsvr/cmd/main.go -conf=config.yml

# Terminal 2: Start Gateway
go run gateway/cmd/main.go -conf=config.yml

# Terminal 3: Start test clients (multiple instances, different player IDs)
go run client/cmd/main.go player1
go run client/cmd/main.go player2
```

## Architecture

This is a game/chat server (`cardwar`) built on **Zinx v1.2.8** — a TCP framework that provides message routing based on message IDs. The module path is `cardwar`.

### Service topology

```
Client (TCP/WS) ───> Gateway ───> ChatSvr (TCP)
                <───         <───
```

- **Gateway** (`gateway/cmd/main.go`): Dual-protocol server (TCP:8999 + WebSocket:9000) facing clients. Maintains an internal TCP client connection to ChatSvr. Wraps incoming client messages in an `Envelope` (routing metadata with `ConnID`) and forwards them to ChatSvr. Routes responses back to the correct client connection, or broadcasts to all clients when `ConnID == 0`.
- **ChatSvr** (`chatsvr/cmd/main.go`): A single TCP server (:8001) handling business logic — login validation, chat message processing, and broadcast generation. Stores logged-in player state in memory (`sync.Map`).
- **Client** (`client/cmd/main.go`): Test client that auto-connects to Gateway, logs in with the given player ID, then sends chat messages on a 5-second loop.

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
- **`router.GatewayRef`** (`gateway_ref.go`): Shared state on Gateway — holds the `ChatSvrTCPConn`, the `IServer` reference (for client connection lookups), and `PlayerConns` (`sync.Map` of playerID → connID).

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

### Configuration

`config.yml` — loaded by `conf/config.go` into `conf.GlobalConfig`. Defines service listen addresses and IDs. Each service uses the first entry in its config array.

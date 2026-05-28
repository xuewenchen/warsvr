# Wire Format

## Zinx DataPack

Zinx v1.2.8 default uses **BigEndian `DataPack`**. Every TCP/WebSocket message is framed:

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

Non-Go clients MUST use BigEndian msgID-first format.

## WebSocket

WebSocket connections use the same DataPack framing. The Gateway's `SetWebsocketAuth` callback handles JWT validation before the upgrade. See the C# example at `examples/csharp/CardWarClient/` for a non-Go client implementation.

## JWT Authentication

Gateway validates JWT tokens before WebSocket upgrade. Token is passed as query param: `ws://host:port/ws?token=<JWT>`.

JWT claims used:
- `playerId` (int64) — internal test token
- `user_id` (int64) — external login server token (fallback)

Gateway's `ValidateJWT` checks both. Signing: HMAC-SHA256.

```go
// Generate (test client)
token, _ := auth.GenerateJWT(playerID, jwtSecret)

// Validate (Gateway)
playerID, err := auth.ValidateJWT(token, jwtSecret)
```

### JWT Flow

1. Client gets JWT from HTTP auth service (or generates locally for testing)
2. Client connects to Gateway: `ws://host:9000/ws?token=<JWT>`
3. Gateway's `SetWebsocketAuth` validates JWT BEFORE WebSocket upgrade
4. Invalid → HTTP 401, connection rejected
5. Valid → `playerId` extracted, stored in `pendingAuths` map (keyed by `RemoteAddr`)
6. `OnConnStart`: picks up from `pendingAuths`, sets `conn.SetProperty("playerId", ...)` and `PlayerConns` mapping
7. No Login/LoginRsp messages forwarded between Gateway and backends

## Envelope (Internal)

Gateway ↔ Backend communication uses `Envelope` (defined in `protocol/proto/cardwar.proto`):

```
Envelope {
  conn_id: uint64           // client session ID (0 = broadcast)
  data: bytes               // protobuf message body
  conn_tags: map<string,string>  // metadata
}
```

Key conn_tags:
- `player_id` — auto-injected by ForwardRouter (sender identity)
- `target_player_id` — triggers private routing in ResponseRouter
- `server_id` — set by MatchSvr, used by DirectRoute for roomsvr
- `match_id` — set by MatchSvr, available to RoomSvr

## C# Client Example

See `examples/csharp/CardWarClient/` for a complete .NET client implementation including:
- BigEndian DataPack framing
- JWT generation via HMAC-SHA256
- ChatReq/ChatResp protobuf serialization
- WebSocket connection to Gateway

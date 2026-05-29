# SessionSvr

## 

Player session persistence — handles disconnect/reconnect and TTL-based cleanup.

- **Session save**: Gateway pushes conn_tags on every backend response that sets connection properties
- **Disconnect marking**: Gateway marks session as disconnected with timestamp (session NOT deleted)
- **Reconnect within TTL (120s)**: Gateway restores conn_tags, notifies RoomSvr to update stale conn reference
- **TTL expiry**: SessionSvr sends force-leave messages to RoomSvr and MatchSvr, then deletes session

## 

```
apps/sessionsvr/cmd/main.go                                # Entrypoint: Dial RoomSvr/MatchSvr, start TTL scanner
apps/sessionsvr/internal/router/
  session_router.go                                         # SessionSave/Get/Disconnect/Reconnect handlers
  expiry.go                                                 # TTL scanner + force-leave cleanup
```

## 

|  |  |
|---|---|
| `pkg` | Registry (Dial RoomSvr/MatchSvr) |
| `protocol` | msgID  |
| `protocol/pb` | SessionData |

## State

- `sessions sync.Map`  `playerId(int64)  *Session`
- `Session.PlayerID`, `GatewayID`, `ConnTags`, `DisconnectedAt` (0=connected)

## 

```bash
scripts\svc.bat start sessionsvr-1
```

## 

### Connect (normal)

```
Gateway  SessionData{player_id, gateway_id, conn_tags}  SessionSvr
   SessionSvr: sessions.Store(playerId, session)
```

### Disconnect

```
Gateway  SessionData{player_id}  SessionSvr
   SessionSvr: session.DisconnectedAt = now
```

### Reconnect (within TTL)

```
Gateway  SessionData{player_id}  SessionSvr
   SessionSvr   SessionData{player_id, conn_tags, disconnectedat=0}
Gateway: restore conn_tags on client connection
Gateway  SessionReconnected  RoomSvr (DirectRoute by server_id)
   RoomSvr: update roomPlayer.conn + roomPlayer.senderID
```

### TTL Expiry ( 120s)

```
SessionSvr ticker (1s):
  for each expired session:
     conn_tags has match_id 
       RouteTo("roomsvr", server_id)  SessionForceLeave
       RoomSvr: remove player, broadcast "left", destroy room if empty
     conn_tags has match_type 
       RouteTo("matchsvr", match_type)  SessionForceLeaveQueue
       MatchSvr: remove player from queue
     sessions.Delete(playerId)
```

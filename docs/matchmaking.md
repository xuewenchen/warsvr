# Matchmaking & Room System

## Services

| Service | Role | MsgIDs |
|---|---|---|
| **MatchSvr** | Matchmaking pool + roomsvr directory (`matchId → serverId`) | 11-13 (pool), 18-21 (directory) |
| **RoomSvr** | Room lifecycle (create/join/leave) | 14-17 |
| **Gateway** | Pure forwarding, config-driven routing | — |

## Protocol

### Match Pool (11-13)

```
MatchEnterReq{match_type, elo}   → enter competitive queue
MatchEnterResp{status, queue_size}
MatchResultPush{match_id, server_id, players, match_type}  → pushed when match ready
```

### Match Directory (18-21)

```
MatchAllocateReq{match_id}       → assign roomsvr for a new match
MatchAllocateResp{match_id, server_id, error}  → conn_tags: {server_id, match_id}

MatchQueryReq{match_id}          → lookup existing match location
MatchQueryResp{match_id, server_id, found}    → conn_tags: {server_id, match_id}
```

### Room Lifecycle (14-17)

```
RoomJoinReq{match_id}            → join room (auto-create on first join)
RoomJoinResp{success, match_id, server_id, error}

RoomLeaveReq{match_id}           → leave room (auto-destroy when empty)
RoomLeaveResp{success, error}
```

## Design Decisions

**MatchSvr does NOT track room players.** It only stores `matchId → serverId`. RoomSvr manages the actual player list. MatchSvr deletes its directory entry when RoomSvr notifies "room destroyed" (empty).

**Allocate vs Query are separate commands.** Creating a room first Allocates (get a roomsvr), then RoomJoins (auto-create). Joining an existing room first Queries (where is it?), then RoomJoins.

**RoomSvr auto-creates on first join.** No separate "create room" command. The first `RoomJoinReq` to a non-existent `match_id` creates the room implicitly.

## Flows

### Create Room

```
1. Client → MatchAllocateReq{match_id:"room-lobby"} → MatchSvr
2. MatchSvr: pick least-loaded roomsvr
3. MatchSvr → MatchAllocateResp{match_id, server_id:"roomsvr-1"}
   conn_tags: {"server_id":"roomsvr-1", "match_id":"room-lobby"}
4. Gateway ResponseRouter: conn.SetProperty("server_id", "roomsvr-1")
5. Client → RoomJoinReq{match_id:"room-lobby"}
   → route_key: server_id → DirectRoute → roomsvr-1
6. RoomSvr: match_id 不存在 → 自动创建房间 → 加入玩家 → RoomJoinResp{success:true}
```

### Join Room

```
1. Client → MatchQueryReq{match_id:"room-lobby"} → MatchSvr
2. MatchSvr: 查表 → server_id:"roomsvr-1"
3. MatchSvr → MatchQueryResp{match_id, server_id:"roomsvr-1", found:true}
   conn_tags: {"server_id":"roomsvr-1", "match_id":"room-lobby"}
4. Gateway: conn.SetProperty("server_id", "roomsvr-1")
5. Client → RoomJoinReq{match_id:"room-lobby"}
   → DirectRoute → roomsvr-1
6. RoomSvr: 房间存在 → 加入 → RoomJoinResp{success:true}
```

### Matchmaking Pool (2v2)

```
1. A → MatchEnterReq{match_type:"2v2"} → queue: [A]   → resp: "waiting"
2. B → MatchEnterReq{match_type:"2v2"} → queue: [A,B] → resp: "waiting"
3. C → MatchEnterReq{match_type:"2v2"} → queue: [A,B,C] → resp: "waiting"
4. D → MatchEnterReq{match_type:"2v2"} → queue: [A,B,C,D] → 4=full → match!

5. MatchSvr: pick roomsvr → create match_id → Save directory
6. MatchSvr → MatchResultPush → All 4 players:
   {match_id:"match-2v2-xxx", server_id:"roomsvr-1", players:[A,B,C,D]}
   conn_tags: {"server_id":"roomsvr-1", "match_id":"match-2v2-xxx"}
7. Each player gets server_id on their connection
8. Each player → RoomJoinReq{match_id} → DirectRoute → roomsvr-1
```

### Leave Room

```
1. Client → RoomLeaveReq{match_id} → DirectRoute → RoomSvr
2. RoomSvr: remove player from room
   - room not empty → RoomLeaveResp{success:true}
   - room empty → destroy room → notify MatchSvr to delete directory → RoomLeaveResp{success:true}
```

## Load Balancing

MatchSvr tracks `loadCounts` (`serverId → int` of active matches). `pickLeastLoaded("roomsvr")` selects the instance with the minimum count. `MatchAllocateReq` and `MatchResultPush` increment the counter.

## Gateway Config

```yaml
gateway:
  routes:
    matchsvr:
      forward: [11, 18, 20]  # enter pool, allocate, query
      route_key: connId       # first request uses hash (stateless)
      route_type: hash
    roomsvr:
      forward: [14, 16]      # room join, room leave
      route_key: server_id    # MatchSvr set this on the connection
      route_type: direct      # precise routing to assigned instance
```

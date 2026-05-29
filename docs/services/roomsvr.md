# RoomSvr

## 功能

房间服务——管理房间生命周期。

- **自动创建**：`RoomJoinReq` 到一个不存在的 matchId 时自动创建房间
- **加入/离开**：玩家进出房间，维护玩家列表
- **自动销毁**：最后一人离开时销毁房间（后续会通知 MatchSvr 删目录记录）

## 目录结构

```
apps/roomsvr/cmd/main.go              # 入口：注册 Join/Leave 路由器
apps/roomsvr/internal/router/
  room_router.go                       # 房间逻辑：自动创建、加入、离开、自动销毁
  ping_router.go
  ping_router.go
```

## 依赖

| 依赖 | 用途 |
|---|---|
| `pkg/broadcast` | Broadcaster：玩家进/出时广播给房间其他人 |
| `protocol` | msgID 常量 |
| `protocol/pb` | RoomJoinReq/Resp, RoomLeaveReq/Resp, Envelope |

## 状态

- `rooms sync.Map` → `matchId → []playerID`（字符串列表，来自 `conn_tags["player_id"]`）

## 依赖

| 依赖 | 用途 |
|---|---|
| `pkg/registry` | Dial MatchSvr，房间销毁时发送通知 |
| `pkg/broadcast` | Broadcaster：玩家进/出时广播给房间其他人 |
| `protocol` | msgID 常量 |
| `protocol/pb` | RoomJoinReq/Resp, RoomLeaveReq/Resp, RoomDestroyedPush, Envelope |

## 启动

```bash
scripts\svc.bat start roomsvr-1
```

## 路由

RoomSvr 通过 Gateway 的 `route_type: direct` 接收请求。客户端已通过 MatchSvr 获得 `server_id`（设为 `roomsvr-1`），Gateway 精准投递到目标实例。

```yaml
gateway:
  routes:
    roomsvr:
      forward: [14, 16]        # room join, room leave
      route_key: server_id     # MatchSvr 分配时设在连接上
      route_type: direct       # 精准匹配 server_id
```

## 交互流程

### 加入房间（首次 = 自动创建）

```
Client → RoomJoinReq{match_id:"room-lobby"} → Gateway → RoomSvr
  (route_key: server_id="roomsvr-1" → DirectRoute)

RoomRouter.handleJoin:
  senderPID = env.ConnTags["player_id"]    // "player-123"

  rooms.LoadOrStore("room-lobby", [])      // 不存在 → 创建空房间
  players = append(players, senderPID)     // ["player-123"]
  rooms.Store("room-lobby", players)

  → resp: RoomJoinResp{success:true, match_id:"room-lobby"}
  → log: "player-123 joined room-lobby (1 players)"
```

### 第二个玩家加入

```
Client B → RoomJoinReq{match_id:"room-lobby"} → Gateway → RoomSvr
  (route_key: server_id="roomsvr-1" → DirectRoute)

RoomRouter.handleJoin:
  senderPID = env.ConnTags["player_id"]    // "player-456"
  rooms.Load("room-lobby") → ["player-123"]
  player-456 不在列表中 → 加入
  players = ["player-123", "player-456"]
  rooms.Store("room-lobby", players)

  → resp: RoomJoinResp{success:true}
  → log: "player-456 joined room-lobby (2 players)"
```

### 离开房间（非最后一人）

```
Client → RoomLeaveReq{match_id:"room-lobby"} → Gateway → RoomSvr

RoomRouter.handleLeave:
  senderPID = env.ConnTags["player_id"]    // "player-123"
  rooms.Load("room-lobby") → ["player-123", "player-456"]
  移除 "player-123"
  players = ["player-456"]   // 非空
  rooms.Store("room-lobby", players)

  → resp: RoomLeaveResp{success:true}
  → log: "player-123 left room-lobby (1 players)"
```

### 离开房间（最后一人 = 销毁）

```
Client → RoomLeaveReq{match_id:"room-lobby"} → Gateway → RoomSvr

RoomRouter.handleLeave:
  senderPID = env.ConnTags["player_id"]    // "player-456"
  rooms.Load("room-lobby") → ["player-456"]
  移除 "player-456"
  players = []   // 空
  rooms.Delete("room-lobby")

  → resp: RoomLeaveResp{success:true}
  → log: "room-lobby destroyed (empty)"
  // TODO: 通知 MatchSvr 删除 activeMatches["room-lobby"]
```

## 与 MatchSvr 的关系

```
MatchSvr                           RoomSvr
─────────                          ────────
matchId → serverId   (目录)        matchId → players (生命周期)
allocate 分配 roomsvr              join 时自动创建
query 查位置                       leave 空时销毁 → 通知 MatchSvr
不跟踪玩家                         不关心服务器分配
```

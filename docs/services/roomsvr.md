# RoomSvr

## 功能

房间服务——管理房间生命周期。

- **自动创建**：`RoomJoinReq` 到一个不存在的 matchId 时自动创建房间
- **加入/离开**：玩家进出房间，维护玩家列表（含 Gateway 连接引用）
- **自动销毁**：最后一人离开时销毁房间，通知 MatchSvr 删除目录记录
- **断线重连**：接收 Gateway 重连通知，更新房间内玩家的连接引用
- **TTL 强制清理**：接收 SessionSvr 强制离开通知

## 目录结构

```
apps/roomsvr/cmd/main.go              # 入口：pkg.NewServer 自动注入 Ping/身份路由
apps/roomsvr/internal/router/
  room_router.go                       # 房间逻辑：自动创建、加入、离开、自动销毁、重连、强制清理
```

## 依赖

| 依赖 | 用途 |
|---|---|
| `pkg` | Broadcaster（玩家进/出/事件广播）、Registry（Dial MatchSvr 通知销毁） |
| `pkg/conf` | 服务名常量、configured servers |
| `protocol` | msgID 常量 |
| `protocol/pb` | RoomJoinReq/Resp, RoomLeaveReq/Resp, RoomDestroyedPush, RoomEventPush, SessionData, Envelope |

## 状态

- `rooms sync.Map` → `matchId → []roomPlayer`
  - `roomPlayer{playerID string, conn ziface.IConnection, senderID uint64}` — playerID 标识玩家，conn 为 Gateway 连接引用（重连时会更新），senderID 为客户端 connID（Envelope 路由用）

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
  → notifyMatchSvr: RouteTo("matchsvr", matchID) → RoomDestroyedPush
```

### 断线重连（Gateway 通知）

```
Gateway → SessionReconnected{player_id, match_id, sender_id} → RoomSvr

RoomRouter.handleReconnected:
  rooms.Load(matchID) → [{player-123, oldConn, oldSenderID}]
  匹配 playerID → 更新 conn = request.GetConnection()（新 Gateway 连接）
  更新 senderID = 新的客户端 connID
  → rooms.Store(matchID, updatedPlayers)
```

重连只更新内部引用（conn、senderID），不广播事件——其他玩家不关心内部状态变化。

### TTL 强制离开（SessionSvr 通知）

```
SessionSvr → SessionForceLeave{player_id, match_id} → RoomSvr

RoomRouter.handleForceLeave:
  rooms.Load(matchID) → 移除该 playerID
  players 非空 → broadcastRoomEvent("left") → 剩余玩家收到离开通知
  players 为空 → rooms.Delete + notifyMatchSvr → 房间销毁
```

### 房间事件广播

玩家进出时，向已有成员广播 `RoomEventPush{match_id, player_id, event("joined"/"left"), player_count}`。通过 `BC.ToConn` 精准投递到每个成员的 Gateway 连接。

## 与 MatchSvr 的关系

```
MatchSvr                           RoomSvr
─────────                          ────────
matchId → serverId   (目录)        matchId → players (生命周期)
allocate 分配 roomsvr              join 时自动创建
query 查位置                       leave 空时销毁 → 通知 MatchSvr
不跟踪玩家                         不关心服务器分配
```

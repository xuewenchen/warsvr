# MatchSvr

## 功能

匹配服务——两件事：**匹配池排队** + **roomsvr 目录**（`matchId → serverId` 映射）。

- 竞技匹配：玩家按 `match_type` 入池，人数凑齐自动匹配并推送结果
- 房间分配：`MatchAllocateReq` 选择负载最低的 roomsvr 实例分配给新 match
- 房间查询：`MatchQueryReq` 查已有 match 在哪个 roomsvr
- 负载均衡：`pickLeastLoaded` 选择 active match 数最少的 roomsvr

**不跟踪房间玩家列表**——那归 RoomSvr 管。

## 目录结构

```
apps/matchsvr/cmd/main.go              # 入口：注册 Enter/Allocate/Query 路由器
apps/matchsvr/internal/router/
  match_router.go                       # 匹配逻辑：队列、分配、查询、匹配池
  ping_router.go
  gateway_register_router.go
```

## 依赖

| 依赖 | 用途 |
|---|---|
| `pkg/broadcast` | Broadcaster（预留，匹配池推送） |
| `pkg/conf` | 读取 roomsvr 实例列表（负载均衡用） |
| `protocol` | msgID 常量 |
| `protocol/pb` | MatchEnterReq/Resp, MatchAllocateReq/Resp, MatchQueryReq/Resp, MatchResultPush |

## 状态

- `activeMatches sync.Map` → `matchId → matchDir{ServerID, MatchType, CreatedAt}`
- `queues sync.Map` → `matchType → []queuedPlayer`
- `loadCounts sync.Map` → `serverId → int`（active match 计数）

## 启动

```bash
scripts\svc.bat start matchsvr-1
```

## 交互流程

### 竞技匹配池

```
A → MatchEnterReq{type:"2v2"} → Gateway → MatchSvr
  → queues["2v2"] = [A]
  → resp: MatchEnterResp{status:"waiting", queue_size:1}

B → MatchEnterReq{type:"2v2"} → MatchSvr
  → queues["2v2"] = [A,B]
  → resp: MatchEnterResp{status:"waiting", queue_size:2}

C → MatchEnterReq{type:"2v2"} → MatchSvr
  → queues["2v2"] = [A,B,C]
  → resp: MatchEnterResp{status:"waiting", queue_size:3}

D → MatchEnterReq{type:"2v2"} → MatchSvr
  → queues["2v2"] = [A,B,C,D] → 4人=满
  → matchPool("2v2", [A,B,C,D]):
    1. pickLeastLoaded("roomsvr") → "roomsvr-1"
    2. matchID = "match-2v2-xxx"
    3. activeMatches[matchID] = {ServerID:"roomsvr-1"}
    4. 向每人推送 MatchResultPush:
       Envelope{ConnId:p.senderID, Data:MatchResultPush, ConnTags:{"server_id":"roomsvr-1"}}
    5. Gateway ResponseRouter: conn.SetProperty("server_id", "roomsvr-1")
  → 每人接着发 RoomJoinReq{matchID} → DirectRoute → roomsvr-1
```

### 房间分配

```
Client → MatchAllocateReq{match_id:"room-lobby"} → Gateway → MatchSvr

MatchRouter.handleAllocate:
  1. activeMatches 中已存在? → 返回 "already exists"
  2. pickLeastLoaded("roomsvr") → "roomsvr-1"
  3. activeMatches["room-lobby"] = {ServerID:"roomsvr-1"}
  4. incLoad("roomsvr-1")
  → resp: MatchAllocateResp{match_id:"room-lobby", server_id:"roomsvr-1"}
    conn_tags: {"server_id":"roomsvr-1", "match_id":"room-lobby"}
  → Gateway: conn.SetProperty("server_id", "roomsvr-1")
```

### 房间查询

```
Client → MatchQueryReq{match_id:"room-lobby"} → Gateway → MatchSvr

MatchRouter.handleQuery:
  1. activeMatches.Load("room-lobby") → 存在 → serverId="roomsvr-1"
  → resp: MatchQueryResp{match_id:"room-lobby", server_id:"roomsvr-1", found:true}
    conn_tags: {"server_id":"roomsvr-1", "match_id":"room-lobby"}
  → Gateway: conn.SetProperty("server_id", "roomsvr-1")
```

## 与 Gateway 的配置

```yaml
gateway:
  routes:
    matchsvr:
      forward: [11, 18, 20]    # enter pool, allocate, query
      route_key: connId
      route_type: hash
```

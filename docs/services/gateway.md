# Gateway

## 功能

客户端入口，负责三件事：**JWT认证**、**协议转发**、**连接管理**。

- JWT 鉴权：WebSocket 升级前验证 token，拒绝无效连接（HTTP 401）
- 纯转发：不解析消息体，根据 config.yml 的路由表将客户端消息转发到对应后端
- 连接映射：维护 `PlayerConns`（playerId → connId），用于私聊精准投递
- 多实例：Gateway 之间无感知，不通信

## 目录结构

```
apps/gateway/cmd/main.go              # 入口：JWT auth, Dial backends, 启动 WS 服务
apps/gateway/internal/router/
  gateway_ref.go                       # GatewayRef（共享状态）, BackendRouteInfo, BuildRouteIndex
  forward_router.go                    # 泛化转发路由器：查路由表 → 打包 Envelope → RouteTo
  response_router.go                   # 泛化响应路由器：解包 Envelope → 应用 conn_tags → 投递
  reconnect.go                         # 断线重连逻辑：CheckReconnect, MarkDisconnected, SyncSessionTags
  session_response.go                  # SessionSvr 响应处理器（SessionGet, SessionReconnect）
```

## 依赖

| 依赖 | 用途 |
|---|---|
| `pkg/auth` | JWT 生成和校验 |
| `pkg` | 连接池（Dial, Route, Sync）、多后端注册管理、HTTPError |
| `pkg/conf` | 配置加载、热加载 |
| `pkg/corouter` | PingRouter（ping→pong） |
| `protocol` | msgID 常量 |
| `protocol/pb` | Envelope, ChatResp（错误响应）, SessionData |
| `pkg/conf` | 服务名常量（`SvcSessionSvr` 等） |

### 断线重连（Session）

Gateway 连接建立后通过 SessionSvr 实现断线重连。核心是**断线不删 session，120s TTL 内重连可恢复状态**。

**流程图：**

```
连接时 (OnConnStart):
  → CheckReconnect(playerID, conn)
    → SessionGet → SessionSvr 返回已存在的 session?
      → 有: HandleSessionGet
        → disconnectedAt != 0 (断线状态) → 恢复 conn_tags → 通知 RoomSvr 更新 conn 引用
        → disconnectedAt == 0 (在线状态) → SyncSessionTags 覆盖
      → 无: 正常新连接

断开时 (OnConnStop):
  → MarkDisconnected(playerID)
    → SessionDisconnect → SessionSvr 标记 disconnectedAt=now（不删除！）

后端响应后 (ResponseRouter.applyConnTags):
  → conn_tags 有 server_id / match_id / match_type → SyncSessionTags → SessionSave
```

**conn_tags 同步逻辑：**

每次后端响应（如 MatchAllocateResp、MatchResultPush）通过 `ResponseRouter` 设置了新的 `conn_tags`（`server_id`、`match_id`）后，Gateway 自动调用 `SyncSessionTags` 把当前连接的关键属性推送到 SessionSvr。这样断线时 SessionSvr 持有完整的上下文（玩家在哪个房间、哪个服务器），TTL 过期时可精确通知 RoomSvr/MatchSvr 清理。

**重连恢复后的通知：**

Gateway 发现 `disconnectedAt != 0`（旧会话存在且处于断线状态）时：
1. 恢复旧 `conn_tags` 到新 WebSocket 连接（`server_id`、`match_id` 等）
2. 通知 SessionSvr 清除断线标记：`SessionReconnect`
3. 通知 RoomSvr 更新 `roomPlayer` 中的 conn 引用：`SessionReconnected`
   - RoomSvr 用新连接替换旧的 Gateway 连接引用
   - 更新 `senderID`（新的 WebSocket connID）

## 启动

```bash
scripts\svc.bat start gw-1
scripts\svc.bat start gw-1 prod.yml gw-1   # 指定配置和实例
```

## 交互流程

### 客户端连接

```
Client ── ws://host:9000/ws?token=<JWT> ──> Gateway
  → SetWebsocketAuth: ValidateJWT(token, secret)
    → 无效: HTTP 401
    → 有效: 提取 playerId → pendingAuths[RemoteAddr] = playerId
  → OnConnStart: playerId → conn.SetProperty("playerId", playerId)
    → gw.PlayerConns.Store(playerId, connID)
```

### 客户端发消息 → 后端

```
Client ── ChatReq(msgID=5) ──> Gateway
  → ForwardRouter:
    1. RouteFor(5) → BackendRouteInfo{Backend:"chatsvr", RouteKey:"playerId", RouteType:"hash"}
    2. resolveRouteKey: conn.GetProperty("playerId") → "12345"
    3. 打包 Envelope{ConnId, Data, ConnTags:{"player_id":"12345"}}
    4. RouteTo("chatsvr", "12345") → HashRoute → chatsvr 连接
    5. conn.SendMsg(5, envData)
```

### 后端响应 → 客户端

```
ChatSvr ── ChatResp(msgID=6, Envelope{ConnId=0, Data}) ──> Gateway
  → ResponseRouter:
    1. 检查 conn_tags["target_player_id"] → 空
    2. ConnId=0 → 遍历所有客户端连接 → SendMsg(6, env.Data)
```

### 私聊投递

```
ChatSvr ── Envelope{ConnId=0, ConnTags:{"target_player_id":"B"}, Data} ──> 所有 Gateway
  → ResponseRouter:
    1. target_player_id="B" → PlayerConns.Load("B") → connID
    2. 仅该 connID 的客户端收到 → SendMsg(6, env.Data)
```

### 新增后端（热加载）

```
config.yml 加 cs-2 → conf.Watch 触发
  → BuildRouteIndex → SetRoutes（原子替换路由表）
  → SyncBackend("chatsvr", routers, HashRoute)
    → Pool.Sync: 对比 ServerAddrs vs config
    → 新增 cs-2 → AddServer → 同步等待连接/超时
```

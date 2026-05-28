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
```

## 依赖

| 依赖 | 用途 |
|---|---|
| `pkg/auth` | JWT 生成和校验 |
| `pkg/pool` | 后端连接池（Dial、Route、Sync） |
| `pkg/registry` | 多后端注册管理 |
| `pkg/errors` | HTTPError |
| `conf` | 配置加载、热加载 |
| `protocol` | msgID 常量 |
| `protocol/pb` | 只用到 `Envelope` 和 `ChatResp`（错误响应） |

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

# Architecture Improvement Todo List

> 基于 2026-06-03 架构评审整理，按优先级排列。

---

## P0 — 致命问题

### 1. 状态无持久化

RoomSvr (`rooms sync.Map`)、MatchSvr (`queues sync.Map`、`activeMatches sync.Map`) 全在内存中。进程崩溃 → 所有房间、匹配队列、目录映射全部丢失。

**建议方案**: WAL 日志或定期快照到本地文件，后续可接入 Redis/RocksDB。

**涉及文件**:
- `apps/roomsvr/internal/router/room_router.go`
- `apps/matchsvr/internal/router/match_router.go`

### 2. proto.Marshal 错误被静默吞掉

`data, _ := proto.Marshal(...)` 模式遍布代码，marshal 失败时发送 nil body，消息被静默丢弃。

**建议方案**: 至少 log 错误，关键路径上应该阻断并返回 error response。

**涉及文件**:
- `pkg/gateway/forward_router.go:48`
- `pkg/broadcast.go:40,50,55`
- `pkg/gateway/reconnect.go` 多处
- `apps/chatsvr/internal/router/chat_router.go`
- `apps/roomsvr/internal/router/room_router.go`
- `apps/matchsvr/internal/router/match_router.go`
- `apps/sessionsvr/internal/router/session_router.go`
- `apps/sessionsvr/internal/router/expiry.go`
- `pkg/corouter/service_identity_router.go`

---

## P1 — 高优先级

### 3. 后端连接无应用层健康检查

`Pool.OnDisconnect` 只在 TCP 断连时触发。后端 hang 住时 TCP 连接仍存活，Gateway 持续向死连接发消息直到 TCP keepalive 超时（通常 2 小时）。

**建议方案**: 对后端连接加应用层心跳超时检测（复用 Ping/Pong 或独立探活）。

**涉及文件**:
- `pkg/pool.go`

### 4. 私聊广播风暴

`ToPlayer` 把私聊消息发给所有 Gateway，再由各 Gateway 自行匹配目标。N 个 Gateway = N 倍流量放大。

**建议方案**: ChatSvr 维护 `playerID → gateway_conn` 映射（从 Envelope 来源连接反推），精准投递到目标所在 Gateway。

**涉及文件**:
- `pkg/broadcast.go`
- `apps/chatsvr/internal/router/chat_router.go`

### 5. OnConnStart 同步阻塞等 SessionSvr

`CheckReconnect` 在 `OnConnStart` 中同步等待最多 3s，SessionSvr 不可达时每个客户端连接都卡 3s。

**建议方案**: 改为异步——先让连接可用，SessionSvr 响应到达后再补设 `conn_tags`。

**涉及文件**:
- `pkg/gateway/reconnect.go`
- `pkg/gateway/server.go`

### 6. ServiceHello 握手失败无恢复

ServiceHello 消息丢失后 ForwardRouter/ResponseRouter 永不注册，只有等 Pool 重连才能恢复（最长 5s 退避）。

**建议方案**: Gateway 发完 SERVICE_IDENTITY 后 N 秒未收到 SERVICE_HELLO，主动重发。

**涉及文件**:
- `pkg/gateway/hello_router.go`
- `pkg/pool.go`

### 7. DirectRoute 无 fallback

RoomSvr 实例下线后，持有旧 `server_id` 的客户端连接路由永久失败，需玩家主动重新 Allocate/Query。

**建议方案**: DirectRoute 匹配失败时回退到 hash 路由，或通知客户端重新分配。

**涉及文件**:
- `pkg/pool.go` — `DirectRoute`
- `pkg/gateway/forward_router.go`

---

## P2 — 中等优先级

### 8. 无连接数/速率限制

同 playerID 可无限建连，单连接无消息频率限制。

**建议方案**:
- 同 playerID 最大连接数限制（如 2，允许断线重连窗口期）
- 单连接 token bucket 限速（如 100 msg/s）

**涉及文件**:
- `pkg/gateway/server.go`

### 9. SessionSvr TTL 全量扫描

每秒遍历全部 sessions 检查过期，数十万在线时 CPU 开销明显。

**建议方案**: 时间轮(time wheel)或分桶，只扫描到期桶。

**涉及文件**:
- `apps/sessionsvr/internal/router/expiry.go`

### 10. 配置热加载竞态窗口 ✅ (2026-06-03)

`SetRoutes` 和 `SyncBackend` 之间，新路由可能引用尚未添加的后端实例，导致 `RouteTo` 返回 nil。

**已修复**: 热加载回调调整为：pool sync → CleanupBackends → SetBackendCfgs → SetRoutes。
同步修复 `Pool.Sync` 始终更新 `routeFn`、新增 `Registry.CleanupBackends` 清理已移除的 backend。

**涉及文件**:
- `pkg/gateway/gateway.go:83-93` — 调整回调执行顺序
- `pkg/pool.go` — `Sync` 末尾更新 `routeFn`；新增 `Close()` 方法
- `pkg/registry.go` — 新增 `CleanupBackends` 方法

### 11. Pool.RemoveServer 不清理僵尸条目

stopped 条目不从 `conns` 切片移除，频繁热加载导致切片膨胀。

**建议方案**: Sync 时 compact 或真删除。

**涉及文件**:
- `pkg/pool.go` — `RemoveServer`、`Sync`

---

## P3 — 建议补充

### 12. 消息可靠性

关键消息（战斗结果、道具发放）发后即忘，客户端断线时直接丢失。

**建议方案**: 关键消息加 ACK，或断线时落库等待重连补推。

### 13. RoomSvr 房间迁移

`route_type: direct` 房间绑定单个 RoomSvr 实例，无法迁移。

**建议方案**: 接受停服清房，或实现房间热迁移。

### 14. 可观测性不足

只有 `zlog` 文本日志，无指标导出。

**建议方案**: 补充连接数、消息吞吐、路由延迟、Session 数量等关键指标导出（Prometheus），Gateway 慢请求日志。

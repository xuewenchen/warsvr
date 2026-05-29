# ChatSvr

## 功能

聊天服务——处理全局广播和私聊。**无状态**，不维护玩家信息。

- 全局聊天：收到 ChatReq → 创建 ChatResp → 遍历所有 Gateway 连接广播
- 私聊：收到 ChatReq → 创建 ChatResp → 通过 `conn_tags["target_player_id"]` 让各 Gateway 自行匹配投递 + 给发送者确认
- 发送者身份从 Envelope `conn_tags["player_id"]` 获取（ForwardRouter 自动注入）

## 目录结构

```
apps/chatsvr/cmd/main.go              # 入口：pkg.NewServer 自动注入 Ping/身份路由
apps/chatsvr/internal/router/
  chat_router.go                       # 聊天逻辑：全局广播、私聊投递+确认
```

## 依赖

| 依赖 | 用途 |
|---|---|
| `pkg` | Broadcaster：ToAll（全局广播）、ToPlayer（精准投递）、ToConn（发送确认） |
| `protocol` | msgID 常量 |
| `protocol/pb` | ChatReq, ChatResp, Envelope |

## 启动

```bash
scripts\svc.bat start cs-1
scripts\svc.bat start cs-1 prod.yml cs-1
```

## 交互流程

### 全局广播

```
Client A → ChatReq{content, target_player_id=0} → Gateway → ChatSvr

ChatRouter:
  senderPID = env.ConnTags["player_id"]    // "1"
  push = ChatResp{sender:1, content, timestamp, target:0}
  BC.ToAll(MsgIdChatResp, pushData)
    → 遍历 s.GetConnMgr() 所有连接
    → 每条连接 SendMsg(MsgIdChatResp, Envelope{ConnId:0, Data})

Gateway-1 收到 → ConnId=0 → 广播给自己所有客户端
Gateway-2 收到 → ConnId=0 → 广播给自己所有客户端
```

### 私聊

```
Client A → ChatReq{content, target_player_id=2} → Gateway → ChatSvr

ChatRouter:
  senderPID = env.ConnTags["player_id"]    // "1"
  push = ChatResp{sender:1, content, timestamp, target:2}

  // 1. 投递给目标（发给所有 Gateway，各自匹配）
  BC.ToPlayer(MsgIdChatResp, 2, pushData)
    → Envelope{ConnTags:{"target_player_id":"2"}, Data}
    → 遍历所有 Gateway 连接发送

  // 2. 确认给发送者（只发给原始连接）
  BC.ToConn(MsgIdChatResp, env.ConnId, pushData, request.GetConnection())
    → Envelope{ConnId:senderConnId, Data}

Gateway-1 收到 target_player_id="2" → PlayerConns.Load(2) → 未找到 → 忽略
Gateway-2 收到 target_player_id="2" → PlayerConns.Load(2) → 找到 → 投递 ✓
Gateway-1 收到 ConnId=A'sConnId → 直接发给 A ✓
```

### 多 Gateway

ChatSvr 作为 TCP 服务端，接收所有 Gateway 的 Dial 连接。`Server.GetConnMgr()` 包含所有 Gateway 连接。Broadcaster 遍历所有连接发送，Gateway 各自决定是否本地投递。

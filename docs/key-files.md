# Key Files Reference

## Gateway

| File | Purpose |
|---|---|
| `apps/gateway/cmd/main.go` | Entrypoint: JWT auth, backend Dial, route setup, hot-reload |
| `apps/gateway/internal/router/gateway_ref.go` | GatewayRef, BackendRouteInfo, BuildRouteIndex |
| `apps/gateway/internal/router/forward_router.go` | Generic client→backend forwarding, route key resolution |
| `apps/gateway/internal/router/response_router.go` | Generic backend→client response handling, conn_tags application |

## ChatSvr

| File | Purpose |
|---|---|
| `apps/chatsvr/cmd/main.go` | Entrypoint |
| `apps/chatsvr/internal/router/chat_router.go` | Chat processing, global/private routing via Broadcaster |

## MatchSvr

| File | Purpose |
|---|---|
| `apps/matchsvr/cmd/main.go` | Entrypoint |
| `apps/matchsvr/internal/router/match_router.go` | Pool queue, allocate roomsvr, lookup match location |

## RoomSvr

| File | Purpose |
|---|---|
| `apps/roomsvr/cmd/main.go` | Entrypoint |
| `apps/roomsvr/internal/router/room_router.go` | Room lifecycle: auto-create on join, auto-destroy on empty |

## Shared Libraries

| File | Purpose |
|---|---|
| `pkg/pool.go` | Backend connection pool: Dial, reconnection, Sync, Add/Remove server |
| `pkg/pool.go` | `RouteFuncFor(type)` — "hash"/"random"/"direct" selection |
| `pkg/registry.go` | Multi-backend Registry: Dial, RouteTo, SyncBackend |
| `pkg/broadcast.go` | Broadcaster: ToAll, ToPlayer, ToConn (filtered by conn_type=gateway) |
| `pkg/auth/jwt.go` | JWT: GenerateJWT, ValidateJWT (HS256, playerId/user_id) |
| `pkg/errors.go` | HTTPError, ErrUnauthorized |

## Config

| File | Purpose |
|---|---|
| `pkg/conf/config.go` | Config types, Load, LookupServer, ParseHostPort |
| `conf/watcher.go` | `Watch(path, callback)` — fsnotify hot-reload |
| `config.yml` | Service instances, JWT secret, gateway routes |

## Protocol

| File | Purpose |
|---|---|
| `protocol/proto/cardwar.proto` | Envelope, ChatReq, ChatResp |
| `protocol/proto/match.proto` | MatchEnterReq/Resp, MatchResultPush, MatchAllocateReq/Resp, MatchQueryReq/Resp |
| `protocol/proto/room.proto` | RoomJoinReq/Resp, RoomLeaveReq/Resp |
| `protocol/proto/msgid.proto` | MsgID enum (source of truth) |
| `protocol/msgid.go` | Go uint32 aliases for pb.MsgID_* |
| `protocol/pb/*.pb.go` | Generated protobuf Go code |

## Tools

| File | Purpose |
|---|---|
| `tools/svchelper/main.go` | Service manager: build/start/stop/restart/reboot + status + jwt |
| `tools/testclient/cmd/main.go` | Go WebSocket test client |
| `tools/loadtest/cmd/main.go` | Load test tool |
| `scripts/svc.sh` / `svc.bat` | Thin wrapper around svchelper |
| `scripts/gen_pb.sh` / `gen_pb.bat` | Protobuf generation |

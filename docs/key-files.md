# Key Files Reference

## Gateway

| File | Purpose |
|---|---|
| `apps/gateway/cmd/main.go` | Entrypoint: minimal — flag parse + `gateway.New()` + Serve |
| `pkg/gateway/gateway.go` | GatewayServer, New(), route table, BackendRouteInfo, BuildRouteIndex, DialSessionSvr |
| `pkg/gateway/server.go` | WebSocket server init: JWT auth, OnConnStart/Stop, Ping/Forward router setup |
| `pkg/gateway/forward.go` | ForwardRouter: client→backend forwarding, route key resolution |
| `pkg/gateway/response.go` | ResponseRouter: backend→client response handling, conn_tags + session sync |
| `pkg/gateway/hello.go` | ServiceHelloRouter: process backend hello, dynamically register ForwardRouter/ResponseRouter |
| `pkg/gateway/reconnect.go` | Reconnect logic: CheckReconnect, MarkDisconnected, SyncSessionTags |
| `pkg/gateway/session.go` | SessionResponseRouter: SessionSvr response handler (SessionGet, SessionReconnect) |

## ChatSvr

| File | Purpose |
|---|---|
| `apps/chatsvr/cmd/main.go` | Entrypoint: `server.New()` + AddRouter + Serve |
| `apps/chatsvr/internal/router/chat_router.go` | Chat processing, global/private routing via Broadcaster |

## MatchSvr

| File | Purpose |
|---|---|
| `apps/matchsvr/cmd/main.go` | Entrypoint: `server.New()` + AddRouter + Serve |
| `apps/matchsvr/internal/router/match_router.go` | Pool queue, allocate roomsvr, lookup match location |

## RoomSvr

| File | Purpose |
|---|---|
| `apps/roomsvr/cmd/main.go` | Entrypoint: `server.New()` + AddRouter + Serve; Dial MatchSvr for room-destroyed |
| `apps/roomsvr/internal/router/room_router.go` | Room lifecycle: auto-create on join, auto-destroy on empty |

## SessionSvr

| File | Purpose |
|---|---|
| `apps/sessionsvr/cmd/main.go` | Entrypoint: `server.New()` with nil msgIDs (session routing is hardcoded); Dial RoomSvr/MatchSvr |
| `apps/sessionsvr/internal/router/session_router.go` | SessionSave/Get/Disconnect/Reconnect handlers; Session struct with sync.RWMutex |
| `apps/sessionsvr/internal/router/expiry.go` | TTL scanner + force-leave cleanup (RWMutex-protected reads) |

## Shared Libraries

| File | Purpose |
|---|---|
| `pkg/server/server.go` | `server.New(cfg, service, forwardIDs, sendIDs)` — wraps znet.NewUserConfServer + auto-registers PingRouter & ServiceIdentityRouter |
| `pkg/gateway/` | All gateway types and initialization — reusable by any gateway project |
| `pkg/pool.go` | Backend connection pool: Dial, reconnection, Sync, Add/Remove server; RouteFunc types; AddConnectionRouter for dynamic registration |
| `pkg/registry.go` | Multi-backend Registry: Dial, RouteTo, SyncBackend, Pool |
| `pkg/corouter/ping_router.go` | Common PingRouter: ping→pong echo, shared by all services |
| `pkg/corouter/service_identity_router.go` | ServiceIdentityRouter: set conn_type on connect + reply ServiceHello (if msgIDs non-empty) |
| `pkg/broadcast.go` | Broadcaster: ToAll, ToPlayer, ToConn (filtered by conn_type=gateway) |
| `pkg/auth/jwt.go` | JWT: GenerateJWT, ValidateJWT (HS256, playerId/user_id) |
| `pkg/errors.go` | HTTPError, ErrUnauthorized |

## Config

| File | Purpose |
|---|---|
| `pkg/conf/config.go` | Config types, Load, LookupServer, ParseHostPort, service name constants |
| `pkg/conf/conf_watcher.go` | `Watch(path, callback)` — fsnotify hot-reload |
| `pkg/connkey/connkey.go` | Well-known conn property / conn_tags key constants (Prop*, Tag*, SyncTagKeys) |
| `config.yml` | Service instances, JWT secret, gateway routes (forward is optional) |

## Protocol

| File | Purpose |
|---|---|
| `protocol/proto/cardwar.proto` | Envelope, ChatReq/Resp, ServiceHello |
| `protocol/proto/match.proto` | MatchEnterReq/Resp, MatchResultPush, MatchAllocateReq/Resp, MatchQueryReq/Resp |
| `protocol/proto/room.proto` | RoomJoinReq/Resp, RoomLeaveReq/Resp |
| `protocol/proto/msgid.proto` | MsgID enum (source of truth for all message IDs) |
| `protocol/msgid.go` | Go uint32 aliases for pb.MsgID_* |
| `protocol/pb/*.pb.go` | Generated protobuf Go code |

## User-Service (gRPC)

| File | Purpose |
|---|---|
| `apps/user/user-service/cmd/main.go` | Entrypoint: YAML config + env var override + gRPC server |
| `apps/user/user-service/config.yml` | Default config (addr, etcd endpoints) |
| `apps/user/user-service/Dockerfile` | Multi-stage Docker build (`FROM scratch`, ~22MB) |
| `apps/user/user-service/internal/handler/user_handler.go` | gRPC handler: Ping, CreateUser, GetUser, UpdateUser, DeleteUser, ListUsers (in-memory store) |
| `apps/user/user-job/cmd/main.go` | NATS consumer + cron ping job (forwards to user-service via gRPC) |
| `protocol/proto/user/user.proto` | Protobuf definition for UserService |
| `protocol/pb/user/user.pb.go` | Generated protobuf Go code |
| `protocol/pb/user/user_grpc.pb.go` | Generated gRPC Go code |

## Docker

| File | Purpose |
|---|---|
| `apps/user/user-service/Dockerfile` | Multi-stage build: Go cross-compile → `FROM scratch` |
| `docker-compose.yml` | Container orchestration for all Dockerized services |
| `.dockerignore` | Exclude non-source files from build context |
| `docs/docker-deploy.md` | Docker deployment guide (build, push, up, down) |

## Tools

| File | Purpose |
|---|---|
| `tools/svchelper/main.go` | Service manager: build/start/stop/restart/reboot + docker-* + status + jwt |
| `tools/testclient/cmd/main.go` | Go WebSocket test client |
| `tools/loadtest/cmd/main.go` | Load test tool |
| `scripts/svc.sh` / `svc.bat` | Thin wrapper around svchelper |
| `scripts/gen_pb.sh` / `gen_pb.bat` | Protobuf generation |

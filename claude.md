# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Rules

- **gofmt**: After writing or editing any `.go` file, run `gofmt -w <file>` before declaring the work complete.

## Project Overview

`cardwar` — a game server built on **Zinx v1.2.8** (TCP/WebSocket framework with msgID-based routing).

### Services

| Service | Role | Routing |
|---|---|---|
| **Gateway** | JWT auth, pure forwarding (config-driven, two generic routers) | Client ↔ Backends |
| **ChatSvr** | Chat processing, global/private broadcast | `route_type: hash` |
| **MatchSvr** | Matchmaking pool + roomsvr directory | `route_type: hash` |
| **RoomSvr** | Room lifecycle (auto-create, auto-destroy) | `route_type: direct` |

### Quick Start

```bash
scripts\svc.bat build all
scripts\svc.bat start cs-1
scripts\svc.bat start gw-1
scripts\svc.bat start matchsvr-1
scripts\svc.bat start roomsvr-1
scripts\svc.bat status    # cluster topology
```

## Documentation

| Doc | Content |
|---|---|
| [docs/services/gateway.md](docs/services/gateway.md) | Gateway: JWT auth, forwarding, routing |
| [docs/services/chatsvr.md](docs/services/chatsvr.md) | ChatSvr: global/private chat, broadcast |
| [docs/services/matchsvr.md](docs/services/matchsvr.md) | MatchSvr: pool queue, allocation, directory |
| [docs/services/roomsvr.md](docs/services/roomsvr.md) | RoomSvr: room lifecycle, auto-create/destroy |
| [docs/architecture.md](docs/architecture.md) | Overall topology, message flow, key types |
| [docs/matchmaking.md](docs/matchmaking.md) | MatchSvr + RoomSvr full flows |
| [docs/configuration.md](docs/configuration.md) | config.yml reference |
| [docs/wire-format.md](docs/wire-format.md) | Zinx DataPack, JWT auth |
| [docs/key-files.md](docs/key-files.md) | File reference table |
| [docs/scripts.md](docs/scripts.md) | Build/start scripts usage |
| [docs/benchmark-2026-05-27.md](docs/benchmark-2026-05-27.md) | Performance benchmark report |

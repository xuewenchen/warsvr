# Scripts

## Unified Service Manager

```
# Linux
./scripts/svc.sh <cmd> <instance> [config]

# Windows
scripts\svc.bat <cmd> <instance> [config]
```

Uses config.yml instance IDs directly. Prefix auto-detects service type:
- `cs-*` = ChatSvr
- `gw-*` = Gateway
- `matchsvr-*` = MatchSvr
- `roomsvr-*` = RoomSvr

### Commands

| cmd | action |
|---|---|
| `build <target>` | Compile binary |
| `start <instance>` | Run from binary (auto-builds if missing) |
| `stop <instance>` | Kill process |
| `restart <instance>` | stop + start |
| `reboot <instance>` | stop + build + start |

### Targets

| target | what it does |
|---|---|
| `cs-1` | ChatSvr instance cs-1 from config |
| `gw-1` | Gateway instance gw-1 from config |
| `gw-2` | Gateway instance gw-2 |
| `matchsvr-1` | MatchSvr instance matchsvr-1 from config |
| `roomsvr-1` | RoomSvr instance roomsvr-1 from config |
| `all` | All instances in config.yml |

### Examples

```bash
# Build
scripts\svc.bat build all              # compile everything

# Start by config ID
scripts\svc.bat start cs-1             # ChatSvr cs-1 from config.yml
scripts\svc.bat start gw-1             # Gateway gw-1 from config.yml
scripts\svc.bat start gw-1 prod.yml    # gw-1 with custom config

# Start all (reads config.yml for all cs-*/gw-* IDs)
scripts\svc.bat start all

# Stop
scripts\svc.bat stop gw-2              # kill gw-2 only
scripts\svc.bat stop all               # kill everything

# Restart / Reboot
scripts\svc.bat restart gw-1           # quick restart
scripts\svc.bat reboot all prod.yml    # full rebuild + restart for prod
```

### Multi-Gateway setup

```yaml
# config.yml
services:
  chatsvr:
    - id: cs-1
      listen: 0.0.0.0:8001
  gateway:
    - id: gw-1
      tcp_listen: 0.0.0.0:8999
      ws_listen: 0.0.0.0:9000
    - id: gw-2
      tcp_listen: 0.0.0.0:8998
      ws_listen: 0.0.0.0:9001
```

```bash
scripts\svc.bat start all              # starts cs-1, gw-1, gw-2
scripts\svc.bat stop gw-2              # kill gw-2 only
scripts\svc.bat restart cs-1           # restart ChatSvr
```

## Web Chat Test

```
test\webchat\index.html
```

Open in browser. Two-player chat panel, supports global + private, multi-Gateway.

## Generate Protobuf

```bash
./scripts/gen_pb.sh   # Linux
scripts\gen_pb.bat    # Windows
```

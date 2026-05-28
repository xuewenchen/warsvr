# Scripts

## Unified Service Manager

```
# Linux
./scripts/svc.sh <cmd> <target> [config] [id]

# Windows
scripts\svc.bat <cmd> <target> [config] [id]
```

### Commands

| cmd | action |
|---|---|
| `build <target>` | Compile binary only |
| `start <target>` | Run compiled binary (auto-builds if missing) |
| `stop <target>` | Kill running process |
| `restart <target>` | stop + start |
| `reboot <target>` | stop + build + start |

### Targets

| target | service |
|---|---|
| `chatsvr` | ChatSvr only |
| `gateway` | Gateway only |
| `all` | Both (starts ChatSvr first, then Gateway) |

### Config & Instance ID

| arg | default | description |
|---|---|---|
| `config` | `config.yml` | Path to config YAML file |
| `id` | first in array | Instance ID matching `services.xxx[].id` in config |

### Examples

```bash
# Build
scripts\svc.bat build all                    # compile everything
scripts\svc.bat build gateway                # compile Gateway only

# Start (single instance, default config)
scripts\svc.bat start chatsvr                # ChatSvr, first instance from config.yml
scripts\svc.bat start gateway                # Gateway, first instance from config.yml
scripts\svc.bat start all                    # both, with 1s gap

# Start (custom config)
scripts\svc.bat start chatsvr prod.yml       # ChatSvr, prod config, first instance
scripts\svc.bat start gateway prod.yml gw-1  # Gateway, prod config, gw-1 instance

# Stop
scripts\svc.bat stop chatsvr                 # kill ChatSvr only
scripts\svc.bat stop all                     # kill both

# Restart (stop + start)
scripts\svc.bat restart chatsvr prod.yml cs-2  # kill cs-2, restart from prod.yml

# Reboot (stop + build + start)
scripts\svc.bat reboot all prod.yml          # full rebuild for prod config
scripts\svc.bat reboot gateway               # rebuild + restart gateway
```

### Multi-Instance (cluster simulation)

Run multiple Gateways or ChatSvrs by passing different IDs:

```bash
# Terminal 1
scripts\svc.bat start chatsvr config.yml cs-1

# Terminal 2
scripts\svc.bat start gateway config.yml gw-1

# Terminal 3 - second Gateway
scripts\svc.bat start gateway config.yml gw-2
```

Requires the config to have matching entries:

```yaml
services:
  gateway:
    - id: gw-1
      tcp_listen: 0.0.0.0:8999
      ws_listen: 0.0.0.0:9000
    - id: gw-2
      tcp_listen: 0.0.0.0:8998
      ws_listen: 0.0.0.0:9001
  chatsvr:
    - id: cs-1
      listen: 0.0.0.0:8001
```

## Generate Protobuf

```bash
./scripts/gen_pb.sh                 # Linux
scripts\gen_pb.bat                  # Windows
```

## Test Clients

```bash
go run ./tools/testclient/cmd/ 1    # Player 1
go run ./tools/testclient/cmd/ 2    # Player 2
```

## Load Test

```bash
go run ./tools/loadtest/cmd/ -c 50 -d 30    # 50 clients, 30 seconds
```

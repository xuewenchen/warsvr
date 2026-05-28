# Scripts

All scripts run from repository root, pass parameters for config and instance ID.

## Generate Protobuf

```bash
# Linux
./scripts/gen_pb.sh

# Windows
scripts\gen_pb.bat
```

Regenerates all `.proto` files in `protocol/proto/` to Go code in `protocol/pb/`. Requires `protoc` and `protoc-gen-go` installed.

## Build

| Platform | Command | Description |
|---|---|---|
| Linux | `./scripts/build.sh` | Build ChatSvr + Gateway |
| Linux | `./scripts/build.sh chatsvr` | Build ChatSvr only |
| Linux | `./scripts/build.sh gateway` | Build Gateway only |
| Windows | `scripts\build.bat` | Build ChatSvr + Gateway |
| Windows | `scripts\build.bat chatsvr` | Build ChatSvr only |

Output: `bin/chatsvr` / `bin/gateway.exe`

## Start Single Service

```
# Linux
./scripts/start_chatsvr.sh                  # default config, first instance
./scripts/start_chatsvr.sh chatsvr-2        # default config, specific ID
./scripts/start_chatsvr.sh config.yml       # custom config, first instance
./scripts/start_chatsvr.sh config.yml cs-1  # custom config, specific ID
example: ./scripts/start_chatsvr.sh config.yml chatsvr-1

# Windows (same syntax)
scripts\start_chatsvr.bat
scripts\start_chatsvr.bat gateway-2
scripts\start_gateway.bat prod.yml gw-1
example: .\scripts\start_chatsvr.bat config.yml chatsvr-1
```

The script auto-detects: if the argument ends with `.yml`/`.yaml` it's a config file, otherwise it's an instance ID. Order doesn't matter — you can pass config only, ID only, or both.

## Start All Services

```
# Linux — background, Ctrl+C to stop
./scripts/start_all.sh [config.yml]

# Windows — separate console windows for each
scripts\start_all.bat [config.yml]
```

## Quick Dev Cycle

```bash
# Linux
./scripts/build.sh && ./scripts/start_all.sh

# Windows
scripts\build.bat && scripts\start_all.bat
```

## Multi-Instance (cluster simulation)

```bash
# Terminal 1: ChatSvr
go run ./apps/chatsvr/cmd/ -conf config.yml -id chatsvr-1

# Terminal 2: Gateway 1
go run ./apps/gateway/cmd/ -conf config.yml -id gateway-1

# Terminal 3: Gateway 2 (requires second gateway entry in config.yml)
go run ./apps/gateway/cmd/ -conf config.yml -id gateway-2

# Terminal 4: Test client
go run ./tools/testclient/cmd/ 1
go run ./tools/testclient/cmd/ 2

# Terminal 5: Load test
go run ./tools/loadtest/cmd/ -c 50 -d 30
```

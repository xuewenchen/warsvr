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
- `sessionsvr-*` = SessionSvr

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
| `sessionsvr-1` | SessionSvr instance sessionsvr-1 from config |
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

## Docker Commands

```
svchelper docker-build <service> [version] [registry]
svchelper docker-push <service> [version] [registry]
svchelper docker-up <service|all>
svchelper docker-down <service|all>
```

### 说明

| 命令 | 功能 |
|---|---|
| `docker-build` | 交叉编译 Linux 二进制 + 构建 Docker 镜像（打上 version + latest 标签） |
| `docker-push` | 推送镜像到 registry |
| `docker-up` | 从 registry 拉最新镜像并启动/更新容器（`--pull always`） |
| `docker-down` | 停止并移除容器 |

### 参数

| 参数 | 默认值 | 说明 |
|---|---|---|
| `service` | （必填） | 服务名，如 `user-service`。自动发现 `apps/<domain>/<svc>/Dockerfile` |
| `version` | `latest` | 镜像版本号，如 `v1.2.0` |
| `registry` | `localhost:5000` | 镜像仓库地址，如 `harbor.company.com` |

### 示例

```bash
# 完整发布流程（默认本地 registry）
svchelper docker-build user-service v1.2.0
svchelper docker-push user-service v1.2.0
svchelper docker-up user-service

# 快速更新（一条命令）
svchelper docker-build user-service   # build 后自动推至 registry
svchelper docker-up user-service      # 拉新镜像、重启容器

# 指定生产仓库
svchelper docker-build user-service v1.2.0 harbor.company.com
svchelper docker-push user-service v1.2.0 harbor.company.com

# 启停
svchelper docker-up user-service      # 启动/更新
svchelper docker-up all               # 启动所有 docker-compose 服务
svchelper docker-down user-service    # 停止并移除
svchelper docker-down all             # 停止所有
```

### docker-compose.yml

仓库根目录的 `docker-compose.yml` 定义容器编排。新增服务的 Docker 支持只需：

1. 创建 `apps/<domain>/<svc>/Dockerfile`
2. 在 `docker-compose.yml` 中添加对应的 service 条目
3. `svchelper docker-build <svc>` 即可

### 环境变量

user-service 镜像支持以下环境变量覆盖 YAML 配置：

| 环境变量 | 默认值 | 说明 |
|---|---|---|
| `LISTEN_ADDR` | `0.0.0.0:50051` | gRPC 监听地址 |
| `ETCD_ENDPOINTS` | `host.docker.internal:2379` | etcd 地址（逗号分隔） |

可在 `docker-compose.yml` 中修改，或用 `-e` 在 `docker run` 时覆盖。

## Generate Protobuf

```bash
./scripts/gen_pb.sh   # Linux
scripts\gen_pb.bat    # Windows
```

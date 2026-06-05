# Docker 部署指南

## 概览

user-service 通过 Docker 容器化部署。使用 `svchelper` 统一管理编译、打包、推送、启动、下线全流程。

```
┌─────────────┐     ┌─────────────┐     ┌──────────────┐
│ go build    │ ──► │ docker build│ ──► │ docker push  │
│ (交叉编译)   │     │ (打包镜像)   │     │ (推送 registry)│
└─────────────┘     └─────────────┘     └──────────────┘
                                              │
                    ┌──────────────┐           │
                    │ docker up/down│ ◄───────┘
                    │ (启停容器)    │
                    └──────────────┘
```

## 前提

- Docker Desktop 已安装并运行
- 本地 registry 已启动（或使用其他 registry）

```bash
# 启动本地 registry（仅首次）
docker run -d --name registry -p 5000:5000 -v registry-data:/var/lib/registry registry:2
```

## 核心文件

| 文件 | 用途 |
|---|---|
| `apps/user/user-service/Dockerfile` | 多阶段构建定义 |
| `apps/user/user-service/cmd/main.go` | 支持 `LISTEN_ADDR` 和 `ETCD_ENDPOINTS` 环境变量覆盖 |
| `docker-compose.yml` | 容器编排（仓库根目录） |
| `.dockerignore` | 排除非必要文件，减小 build context |

## svchelper Docker 命令

所有命令都从仓库根目录执行。

### docker-build — 交叉编译 + 打包镜像

```bash
# 推送到本地 registry（默认 localhost:5000）
svchelper docker-build user-service
svchelper docker-build user-service v1.2.0

# 推送到生产 registry
svchelper docker-build user-service v1.2.0 harbor.company.com
```

`docker-build` 做了什么：

1. `GOOS=linux CGO_ENABLED=0 go build` → `bin/user-service-linux`（静态二进制）
2. `docker build -f apps/user/user-service/Dockerfile -t <registry>/cardwar/user-service:<version> -t <registry>/cardwar/user-service:latest .`

镜像大小约 22MB，基于 `FROM scratch`，不依赖任何基础镜像。

### docker-push — 推送镜像到 registry

```bash
svchelper docker-push user-service         # 推 latest
svchelper docker-push user-service v1.2.0  # 推指定版本
```

### docker-up — 启动/更新容器

```bash
svchelper docker-up user-service  # 拉最新镜像并启动/更新单个服务
svchelper docker-up all           # 启动 docker-compose.yml 中所有服务
```

每次执行都会 `--pull always` 自动拉取最新镜像，然后替换旧容器（rolling update）。

### docker-down — 停止并移除容器

```bash
svchelper docker-down user-service  # 停止并移除单个服务
svchelper docker-down all           # 停止并移除所有服务
```

## 完整发布流程

```bash
# 1. 交叉编译 + 打包（默认推送到 localhost:5000）
svchelper docker-build user-service v1.2.0

# 2. 推送镜像
svchelper docker-push user-service v1.2.0

# 3. 更新线上容器（自动拉最新镜像、停旧启新）
svchelper docker-up user-service
```

如果 registry 已在 build 时嵌入标签，第 2 步可省略（build 后镜像已就绪）。

## 日常更新（一条命令）

代码改动后，只想快速更新：

```bash
svchelper docker-build user-service  # build → registry
svchelper docker-up user-service     # 拉新镜像、重启容器
```

## 环境变量

镜像内置了合理的默认值，运行时可通过环境变量覆盖：

| 环境变量 | 默认值 | 说明 |
|---|---|---|
| `LISTEN_ADDR` | `0.0.0.0:50051` | gRPC 监听地址 |
| `ETCD_ENDPOINTS` | `host.docker.internal:2379` | etcd 地址（逗号分隔多个） |

### Docker Compose 中覆盖

编辑 `docker-compose.yml`：

```yaml
services:
  user-service:
    image: localhost:5000/cardwar/user-service:latest
    ports:
      - "50051:50051"
    environment:
      - ETCD_ENDPOINTS=etcd:2379        # etcd 也在 docker 里
      - LISTEN_ADDR=0.0.0.0:50051
```

### 命令行覆盖

```bash
docker run -d --name user-service \
  -p 50051:50051 \
  -e ETCD_ENDPOINTS=10.0.0.5:2379 \
  localhost:5000/cardwar/user-service:latest
```

## 添加新服务的 Docker 支持

1. 在服务目录下创建 `Dockerfile`
2. 在 `docker-compose.yml` 中添加服务定义
3. 运行 `svchelper docker-build <service>`

`svchelper` 会自动扫描 `apps/<domain>/<service>/cmd/` 和 `apps/<service>/cmd/` 路径来发现服务。

## 验证

```bash
# 查 registry 中有哪些版本
curl -s http://localhost:5000/v2/cardwar/user-service/tags/list

# 确认镜像能被拉取（删本地后重新拉）
docker rmi localhost:5000/cardwar/user-service:v1.2.0
docker pull localhost:5000/cardwar/user-service:v1.2.0

# 查看容器状态
docker ps --filter name=user-service
```

## 常见问题

### Docker Hub 无法访问

如果无法拉取 `golang` 基础镜像，`FROM scratch` 方案完全避开了这个问题——镜像构建不需要任何网络拉取。

### 交叉编译失败

确保本机 Go 版本与 `go.mod` 中的版本一致（当前 `go 1.26.2`）。

### 容器启动后 etcd 连接失败

- 检查 `ETCD_ENDPOINTS` 是否指向正确的地址
- 从容器访问宿主机 etcd 使用 `host.docker.internal:2379`
- 从容器访问容器内 etcd 使用容器名（如 `etcd:2379`）

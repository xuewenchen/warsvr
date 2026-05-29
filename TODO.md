# TODO — Future Plans & Improvements

## etcd Service Discovery

Current config is loaded from `config.yml` via `conf.Load(path)` into a global singleton (`conf.GlobalConfig`).
Future etcd integration replaces this with a `Provider` interface that both file and etcd implement.

### Current Coupling Points

```
Pool.Dial → conf.GlobalConfig.Services["matchsvr"]
Pool.Sync → conf.GlobalConfig.Services[service]
Gateway   → conf.GlobalConfig.Gateway.Routes
```

### Plan: Provider Abstraction

**1. Define an interface** (`pkg/conf/provider.go`):

```go
type ServiceDiscovery interface {
    Services() map[string][]ServerNode
    OnChange(callback func(service string, nodes []ServerNode)) func()
}

type Provider interface {
    ServiceDiscovery
    GatewayConfig() GatewayConfig
    OnGatewayChange(callback func(GatewayConfig)) func()
}
```

**2. Two implementations:**

| Implementation | Data Source | Watch Mechanism |
|---------------|-------------|-----------------|
| `FileProvider` | config.yml | fsnotify (existing) |
| `EtcdProvider` | etcd `/services/*` | etcd watch |

**3. Unified startup:**

```
Provider = "file" | "etcd"
  → Services()          → Pool.Dial / Pool.Sync
  → GatewayConfig()     → BuildRouteIndex
  → OnChange()          → Pool.Sync / SetRoutes
```

**4. etcd auto-registration:** Services self-register on startup with TTL lease.
Process crash kills lease → auto-unregister. Each service knows its own identity
from the existing `--id` flag (e.g. `--id cs-1`); the listen address comes from
the service's own zinx config or a dedicated port argument, not from config.yml.

```bash
./chatsvr --id cs-1 --discovery etcd --etcd 127.0.0.1:2379
```

### Migration Steps

| Phase | Change | Impact |
|-------|--------|--------|
| 1 | Add `Provider` interface + `FileProvider` impl | Replace `conf.GlobalConfig` with `conf.Provider`, existing behavior unchanged |
| 2 | Add `EtcdProvider` + `--discovery etcd` flag | Single-node testing, default stays `file` |
| 3 | Pool.Dial/Sync uses Provider | Both modes transparent |
| 4 | Self-registration + TTL lease | Crash → auto-removal, no manual config.yml edits |

### Compatibility

```go
// Before (current)
conf.Load("config.yml")
servers := conf.GlobalConfig.Services["chatsvr"]
conf.Watch(path, callback)

// After (file mode — same behavior)
provider := conf.NewFileProvider("config.yml")
servers := provider.Services()["chatsvr"]
provider.OnChange(callback)

// After (etcd mode)
provider := conf.NewEtcdProvider("localhost:2379")
servers := provider.Services()["chatsvr"]    // same call site, different source
provider.OnChange(callback)                   // same callback, etcd watch under the hood
```

reg.Dial` signature unchanged — it reads from Provider, agnostic to file vs etcd.

### CLI Interface

```bash
# File mode (default — same as today, no flags needed)
./chatsvr --id cs-1

# etcd mode (opt-in)
./chatsvr --id cs-1 --discovery etcd --etcd 127.0.0.1:2379
```

`--discovery` defaults to `"file"`; when set to `"etcd"`, `--etcd` specifies the cluster address.

### svchelper Adaptations

| Command | File Mode (current) | etcd Mode | Change |
|---------|---------------------|-----------|--------|
| `build` | Compile binaries | Same | None |
| `start` | `./chatsvr --id cs-1 --conf config.yml` | `./chatsvr --id cs-1 --discovery etcd --etcd <addr>` | Pass mode-specific flags |
| `stop` | Kill by PID | Same | None |
| `status` | Parse config.yml for instance list | Query etcd for registered services | Add etcd client query |
| `jwt` | Generate token | Same | None |

Mode can be auto-detected by convention: if `--discovery etcd` was passed to `start`,
store the etcd address in the PID file. `status` reads it and decides whether to
parse config.yml or query etcd.

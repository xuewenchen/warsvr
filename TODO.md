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
Process crash kills lease → auto-unregister.

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

`reg.Dial` signature unchanged — it reads from Provider, agnostic to file vs etcd.

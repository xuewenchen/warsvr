# Performance Benchmark Report — 2026-05-27

## Environment

- CPU: Intel Core i5-13600KF (20 threads)
- OS: Windows 11
- Go: amd64
- Zinx: v1.2.8

## E2E Benchmarks

Single Gateway + Single ChatSvr, localhost.

| Benchmark | Metric | Value | Notes |
|---|---|---|---|
| `BenchmarkE2E_Latency` | latency | **85.9 µs/op** | Single client round-trip: ChatReq → Gateway → ChatSvr → Gateway → ChatResp |
| `BenchmarkE2E_Throughput` | combined | **27,000 msg/s** | 10 concurrent clients, 41.5 µs/op avg |
| `BenchmarkE2E_BroadcastMass` | broadcast | **2.06 ms/round** | 50 connected clients, single sender broadcasts to all |

Equivalent throughput:
- Single client: ~11,600 msg/s
- 50-person broadcast: ~485 broadcasts/s
- Per-client broadcast delivery: ~24,250 msg/s (50 × 485)

## Message Reliability

| Test | Messages | Result |
|---|---|---|
| `TestE2E_NoMessageLoss` | 1,000 global | Zero loss, zero duplicates |
| `TestE2E_NoMessageLoss_Private` | 500 private | Zero loss, zero duplicates |
| `TestE2E_GlobalChat` | e2e functional | Both clients receive global chat |
| `TestE2E_PrivateChat` | e2e functional | Target receives, sender gets confirmation, no leak |
| `TestE2E_MultiGW_SingleCS` | 2 GW + 1 CS | Cross-Gateway global + private |
| `TestE2E_SingleGW_MultiCS` | 1 GW + 2 CS | Multi-ChatSvr routing |
| `TestE2E_MultiGW_MultiCS` | 2 GW + 2 CS | Full mesh topology |
| `TestE2E_ThreeGateways_OneChatSvr` | 3 GW + 1 CS | Three Gateways stress |
| `TestE2E_InvalidJWT` | auth | Invalid token rejected |
| `TestE2E_ChatSvrRestart` | resilience | Gateway auto-reconnects within 5s |

## Bottleneck Analysis

1. **Protobuf double-serialization**: Every chat message is serialized as `ChatReq`, wrapped in `Envelope`, then `ChatResp` wrapped in another `Envelope`. Each hop unmarshals both layers. This accounts for ~60% of the 85µs latency (based on alloc count: 42 allocs/op).

2. **WebSocket per-message overhead**: Each broadcast sends a separate WebSocket frame per client. With 50 clients, that's 50 separate `conn.SendMsg` calls per broadcast round.

3. **Zinx worker pool**: Default 10 workers. Under high concurrency, message processing may queue.

## Optimization TODO

### P0 — High Impact, Low Effort

- [ ] **Bypass Envelope unwrap for local Gateway delivery**: When ResponseRouter sends to a local client, skip re-marshaling the Envelope. The `env.Data` is already the final `ChatResp` bytes — send them directly without wrapping.

- [ ] **Batch broadcast send**: Instead of N separate `SendMsg` calls for N clients, collect `ChatResp` bytes once and iterate. Consider a `SendMsgToAll` primitive that avoids per-client allocations.

### P1 — Medium Impact

- [ ] **Connection pool preheating**: Gateway currently pauses on startup waiting for all backend Dials. Add a readiness probe so the Gateway can accept client connections immediately while backends connect in the background.

- [ ] **Protobuf buffer reuse**: Use `sync.Pool` for `proto.Marshal` buffers to reduce GC pressure (currently 42 allocs/op, 2,350 B/op).

- [ ] **Worker pool sizing**: Make Zinx worker pool size configurable. Benchmark with 20/50/100 workers to find the sweet spot for the hardware.

### P2 — Lower Impact, Higher Effort

- [ ] **Zero-copy Envelope**: Embed `ChatResp` bytes directly in Envelope without intermediate marshal/unmarshal. Pass raw bytes through the pipeline, only deserialize when needed by business logic.

- [ ] **WebSocket write coalescing**: Buffer multiple outgoing WebSocket frames and flush in batches. Particularly impactful for broadcast scenarios.

- [ ] **Connection multiplexing**: Single TCP connection between Gateway and ChatSvr instead of one per service instance. Reduces connection management overhead.

## How to Run

```bash
# Unit + integration tests
go test ./...

# E2E tests (requires binary builds)
go test -v ./test/e2e/

# E2E benchmarks
go test -bench=BenchmarkE2E -benchtime=3s ./test/e2e/

# Standalone load test (requires running Gateway + ChatSvr first)
go run ./tools/loadtest/cmd/ -c 50 -d 30 -secret "change-me-in-production"

# Broadcaster benchmarks
go test -bench=. -benchmem ./pkg/
```

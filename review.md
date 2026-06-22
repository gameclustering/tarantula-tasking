# Clustering Sync Code Review — 2026-06-21

Files reviewed:
- `app/web/postoffice/clustering/member_list_manager.go`
- `app/web/postoffice/clustering/member_list_listener.go`
- `app/web/postoffice/clustering/data_service_updater.go`

---

## Findings

### Medium

**`SYNC_NODE_OPT` still uses UDP** (`member_list_listener.go:93`)

Data range handoff on node join uses `m.SendToAddress()` (UDP). If dropped, the joining node silently
misses historical data for its ring range. Eventually self-heals as new writes arrive at the correct
node, but the gap can be large. Same class of problem as the subscription sync bug fixed in v2.8.3.
Fix mirrors the subscription fix — use `go m.SendReliable()`:

```go
case SYNC_NODE_OPT:
    for _, mbr := range m.Members() {
        if mbr.Address() == mr.Address {
            go m.SendReliable(mbr, util.ToJson(mr.Source))
            break
        }
    }
```

---

### Low

**Subscription push only targets `Nodes[0]`** (`data_service_updater.go:99`)

In `balanceOnNodeAdded`, the ring range loop iterates all `added.Nodes`, but the subscription
sync hardcodes `added.Nodes[0].IP` as the target:

```go
subReq := core.RingRequest{..., Address: added.Nodes[0].IP, ...}
```

If memberlist ever fires a single `NodeJoin` event with multiple nodes (e.g., after a partition
heals), only the first node gets subscriptions pushed. In practice memberlist fires one event per
node so `len(added.Nodes) == 1` always holds, but this implicit assumption is not guarded.
Consider iterating `added.Nodes` in the subscription sync loop as well, or adding a guard/comment.

---

**`m.running = false` data race** (`member_list_manager.go:113`)

`ShutdownHook()` sets `m.running = false` directly at line 113, while `RingUpdated()` also sets
it when it receives `NODE_STATE_SHUTDOWN` (line 300 of `data_service_updater.go`). Two goroutines
writing the same field without synchronization is flagged by Go's `-race` detector. Not a real
correctness issue since shutdown is a one-way transition, but the simplest fix is to remove the
direct set at line 113 and rely solely on the channel-driven path (line 119 already sends
`NODE_STATE_SHUTDOWN` to trigger it).

---

**Targeted `SYNC_SUB_OPT` send blocks `Listen()`** (`member_list_listener.go:102`)

The addressed (single-peer) branch of `SYNC_SUB_OPT` calls `m.SendReliable()` synchronously,
blocking `Listen()` for the TCP send duration (~5–10ms). On node join with 25 local subscriptions
this accumulates to ~250ms where no ring events or broadcasts are processed. The broadcast branch
already uses `go m.SendReliable()` — apply the same to the targeted branch for consistency:

```go
go m.SendReliable(mbr, util.ToJson(mr.Source))
```

---

**Periodic sync goroutines can accumulate** (`data_service_updater.go:167`)

Every 60s tick spawns a goroutine that iterates subscriptions and sends to `MRequest`. If
`MRequest` is backed up for a full tick interval, a second goroutine starts before the first
finishes. Both concurrently call `m.subscriptions.lookup()`. Verify that `lookup()` holds its own
lock (read lock is sufficient). If subscriptions uses `sync.RWMutex` with a read lock in `lookup`,
concurrent goroutines are safe.

---

### Info

**`lk` sentinel logic in TRANS_MAIL loop lacks a comment** (`data_service_updater.go:288`)

The variable `lk` is used to detect when the entire `listenerPool` has been iterated without
finding a subscriber. The logic is correct but non-obvious: `lk` is set to the current key before
rotating it to the tail; if the next head equals `lk`, the full pool was scanned. A short comment
would help future readers.

---

## What the v2.8.3 fixes got right

- **`balanceOnNodeAdded` early-return fix**: separating data range handoff (conditional on
  `len(ringSync.Ranges) > 0`) from subscription sync (unconditional) is the correct structural
  change. The comment explaining why subscription sync must always run is good.
- **UDP → TCP for subscription broadcasts**: switching from `SendToAddress` to `go SendReliable`
  per peer is the right fix. Pre-serializing `payload` once and sharing the read-only bytes across
  goroutines is a correct optimization.
- **`select/default/goroutine` pattern**: used consistently throughout `balanceOnNodeAdded` for
  non-blocking sends with bounded goroutine fallback. Buffer size (256) is well above the typical
  subscription count.
- **Jitter on first periodic sync tick**: random 1–60s first tick correctly spreads cluster-wide
  subscription broadcasts after a full rotation.

---

## Priority order for follow-up

1. `SYNC_NODE_OPT` → TCP (medium risk, straightforward fix)
2. Verify `subscriptions.lookup()` locking (confirm safety, no code change likely needed)
3. Targeted `SYNC_SUB_OPT` → goroutine (low risk, one-line change)
4. `Nodes[0]` assumption → document or guard (low risk)
5. `m.running` race → remove direct set in `ShutdownHook` (cosmetic)

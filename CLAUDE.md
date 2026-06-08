# tzla-ble

BLE adapter interface and implementations for connecting to Tesla vehicles via BLE.
Wraps the Tesla [vehicle-command](https://github.com/teslamotors/vehicle-command) library with a swappable adapter interface (proposed in [PR #400](https://github.com/teslamotors/vehicle-command/pull/400), not yet merged upstream).

## What this package does

- Defines the `Adapter`/`Device`/`Service`/`Writer` interfaces (`iface.go`)
- Implements `Connection` which satisfies `vehicle.Connector` from vehicle-command (`ble.go`)
- Provides two BLE backends: `tinygo/` (Linux via BlueZ/D-Bus) and `goble/` (cross-platform via go-ble)
- Used by `/home/sbe/hack/meda/golang/tesla/ble-commander` via the `BleExecutor` in `internal/control/ble.go`

## Key design notes

### RX path
- `service.Rx(uuid, conn.rx)` registers a BLE notification callback
- `rx()` assembles BLE chunks into complete framed messages (2-byte big-endian length prefix)
- Complete messages are pushed to `conn.inbox` (buffered channel, default 100, overridable via `BLE_INBOX_SIZE`)
- `rxLock` serializes concurrent notification callbacks
- On inbox full: oldest message is evicted to make room for the newest (ensures command responses always get through even when stale unsolicited notifications have filled the buffer)
- `bytes.Clone` is used when slicing from `inputBuffer` to avoid a data race on the backing array

### TX path
- `Send()` frames the buffer (2-byte length prefix) and writes it in MTU-sized chunks
- `lock` serializes concurrent sends and prevents send/close races
- BLE writes use `WriteWithoutResponse` (tinygo/Linux) — fire-and-forget at the ATT layer, but the underlying D-Bus call can still block. If BlueZ hangs, the write blocks indefinitely holding `lock`. The watchdog timers in `ble-commander/internal/control/ble.go` are the backstop for this.
- Messages > `maxBLEMessageSize` (1024 bytes) are rejected upfront by `Send()`

### Inbox debug metric
Set `BLE_INBOX_DEBUG=1` to enable debug logging of the inbox high-water mark. Logs a `DEBUG` line each time the inbox reaches a new peak depth, with `hwm` (current peak) and `cap` (configured size).

## Env vars

| Variable | Default | Description |
|---|---|---|
| `BLE_INBOX_SIZE` | `100` | Inbox channel buffer size |
| `BLE_INBOX_DEBUG` | off | Enable inbox high-water mark debug logging |

## Upstream reference

Do not look at local clones for the vehicle-command API — use the upstream source:
https://github.com/teslamotors/vehicle-command.git

The relevant connector interface is `pkg/connector/connector.go`. `Connection` in `ble.go` implements it.

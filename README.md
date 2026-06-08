# tzla-ble

**Minimal BLE adapter interface and implementations, based on [vehicle-command#400](https://github.com/teslamotors/vehicle-command/pull/400) (unmerged), for easy integration and runtime selection.**

This package lets you use either `go-ble` or `tinygo` BLE implementations via a common interface, as proposed in the PR above, **before it is merged upstream**.

---

## Motivation

The [vehicle-command](https://github.com/teslamotors/vehicle-command) repository currently does not allow runtime selection between BLE implementations. This repo extracts the interface and adapters from [PR #400](https://github.com/teslamotors/vehicle-command/pull/400), so you can:

- Choose and switch between **TinyGo** and **go-ble** BLE backends at runtime
- Integrate with your vehicle-command based code now, **before upstream merge**

---

## Usage

### 1. Install

```shell
go get github.com/zlymeda/tzla-ble
```

### 2. Select Adapter in your code

```go
package main

import (
	"context"
	"time"

	"github.com/teslamotors/vehicle-command/pkg/cache"
	"github.com/teslamotors/vehicle-command/pkg/vehicle"
	"github.com/zlymeda/tzla-ble"
	"github.com/zlymeda/tzla-ble/tinygo"
)

func main() {
	adapter, _ := tinygo.NewAdapter("") // or goble.NewAdapter("")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	vin := "your_vehicle_vin_here"

	adv, _ := ble.ScanVehicleBeacon(ctx, vin, adapter)
	conn, _ := ble.NewConnectionFromBeacon(ctx, vin, adv, adapter)
	car, _ := vehicle.NewVehicle(conn, loadPrivateKey(), cache.New(0))

	// use car...
}
```

### 3. Dynamic Backend Selection

```go
func PickAdapter(name string) (ble.Adapter, error) {
	switch name {
	case "tinygo":
		return tinygo.NewAdapter("")
	default:
		return goble.NewAdapter("")
	}
}
```

---

## Configuration

Both settings are read at startup from environment variables.

| Variable | Default | Description |
|---|---|---|
| `BLE_INBOX_SIZE` | `100` | RX inbox buffer depth (number of complete framed messages) |
| `BLE_INBOX_DEBUG` | off | Set to any non-empty value to log inbox high-water mark at DEBUG level |

### Inbox behaviour

Incoming BLE notifications are assembled into framed messages and queued in a buffered channel (the inbox). If the inbox fills up (e.g. the car sends unsolicited status notifications faster than the consumer drains them), the **oldest** message is evicted to make room for the newest. This ensures command responses always arrive even when the buffer is full of stale notifications. A warning is logged on each eviction.

Set `BLE_INBOX_DEBUG=1` to observe the actual peak fill depth — useful for sizing `BLE_INBOX_SIZE`.

---

## API

- [`iface.go`](./iface.go) — `Adapter`, `Device`, `Service`, `Writer` interfaces
- [`ble.go`](./ble.go) — `Connection` (implements `vehicle.Connector`)
- [`tinygo/`](./tinygo/) — BLE backend via [tinygo/bluetooth](https://github.com/tinygo-org/bluetooth) (Linux/BlueZ)
- [`goble/`](./goble/) — BLE backend via [go-ble](https://github.com/zlymeda/go-ble)

---

## Why not just use `vehicle-command`?

The upstream repo does not yet support runtime BLE adapter selection. This repo enables integration until [PR #400](https://github.com/teslamotors/vehicle-command/pull/400) is merged.

---

## License

Apache 2.0, same as [vehicle-command](https://github.com/teslamotors/vehicle-command).

**Attribution:** Based on the work in [vehicle-command#400](https://github.com/teslamotors/vehicle-command/pull/400) and the original Tesla Motors project.

---

**Links:**
- [vehicle-command](https://github.com/teslamotors/vehicle-command)
- [Pull Request #400](https://github.com/teslamotors/vehicle-command/pull/400)
- [go-ble fork](https://github.com/zlymeda/go-ble)
- [TinyGo BLE](https://github.com/tinygo-org/bluetooth)

*This repository is not affiliated with Tesla or the original vehicle-command developers.*

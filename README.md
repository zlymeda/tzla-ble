# tzla-ble

**Minimal BLE adapter interface and implementations, based on [vehicle-command#400](https://github.com/teslamotors/vehicle-command/pull/400) (unmerged), for easy integration and runtime selection.**

This package lets you use either `go-ble` or `tinygo` BLE implementations via a common interface, as proposed in the MR above, **before the PR is merged upstream**.

---

## Motivation

The [vehicle-command](https://github.com/teslamotors/vehicle-command) repository currently does not allow runtime selection between BLE implementations. This repo extracts the interface and adapters from the [MR #400](https://github.com/teslamotors/vehicle-command/pull/400), so you can:

- Choose and switch between **TinyGo** and **go-ble** BLE backends at runtime
- Integrate with your vehicle-command based code now, **before upstream merge**
- Help develop and test the adapter interface cross-platform

---

## Features

- Unified **BLE Adapter Interface**
- Plug-and-play implementations using **go-ble** and **tinygo**
- Trivial runtime backend selection
- Will track and adapt to any upstream changes in MR #400

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
	"github.com/zlymeda/tzla-ble/goble"
)

// ignoring error handling for brevity
func main() {
	var adapter ble.Adapter
	// Example: select go-ble implementation
	adapter, _ = goble.NewAdapter("")

	// Or: use tinygo implementation
	// adapter = tinygo.NewAdapter("")

	// Use adapter as needed...
	requestCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	vin := "your_vehicle_vin_here"

	adv, _ := ble.ScanVehicleBeacon(requestCtx, vin, adapter)

	conn, _ := ble.NewConnectionFromBeacon(requestCtx, vin, adv, adapter)
    
	
	car, _ := vehicle.NewVehicle(conn, loadPrivateKey(), cache.New(0))
	
	// now you can use car as needed
}

```

### 3. Dynamic Backend Selection

```go
package main

import (
	"github.com/zlymeda/tzla-ble"
	"github.com/zlymeda/tzla-ble/goble"
	"github.com/zlymeda/tzla-ble/tinygo"
)

func PickAdapter(name string) (ble.Adapter, error) {
    switch name {
        case "tinygo":
            return tinygo.NewAdapter("")
        default:
            return goble.NewAdapter("")
    }
}
```

## API

- See [`iface.go`](./iface.go) for core interface.
- See [`goble/`](./goble/) and [`tinygo/`](./tinygo/) for concrete implementations.

---

## Why not just use `vehicle-command`?

The upstream repo does *not yet* support runtime BLE adapter selection (as of this writing). This standalone repo enables experimentation and integration until [MR #400](https://github.com/teslamotors/vehicle-command/pull/400) is merged.

---

## Contributing

Feedback and PRs are welcome—especially for additional BLE implementations, documentation, or compatibility testing.

---

## License

Apache 2.0, same as [vehicle-command](https://github.com/teslamotors/vehicle-command).

**Attribution:**  
Based on the work in [vehicle-command#400](https://github.com/teslamotors/vehicle-command/pull/400) and the original Tesla Motors project.

---

**Links:**
- [Pull Request #400 (original interface proposal)](https://github.com/teslamotors/vehicle-command/pull/400)
- [go-ble fork reference](https://github.com/zlymeda/go-ble)
- [TinyGo BLE](https://github.com/tinygo-org/bluetooth)

*This repository is not affiliated with Tesla or the original vehicle-command developers.*

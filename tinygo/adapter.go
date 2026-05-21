package tinygo

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/teslamotors/vehicle-command/pkg/protocol"
	"github.com/zlymeda/tzla-ble"
	"tinygo.org/x/bluetooth"
)

const connectGracePeriod = 5 * time.Second

var ErrAdapterInvalidID = protocol.NewError("the bluetooth adapter ID is invalid", false, false)

func NewAdapter(id string) (ble.Adapter, error) {
	device, err := newAdapter(id)
	if err != nil {
		return nil, fmt.Errorf("ble: failed to create device: %s", err)
	}
	if err = device.Enable(); err != nil {
		return nil, fmt.Errorf("ble: failed to enable device: %s", err)
	}

	return &adapter{
		device: device,
	}, nil
}

type adapter struct {
	device *bluetooth.Adapter
}

func (s *adapter) ScanBeacon(ctx context.Context, name string) (*ble.Beacon, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	stopScan := func() {
		err := s.device.StopScan()
		if err != nil {
			if strings.Contains(err.Error(), "no scan in progress") {
				return
			}
			if strings.Contains(err.Error(), "not calling Scan function") {
				return
			}
			slog.Warn("ble: failed to stop scan", slog.Any("error", err))
		}
	}

	scanCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		<-scanCtx.Done()
		stopScan()
	}()

	resultCh := make(chan *ble.Beacon, 1)
	err := s.device.Scan(func(_ *bluetooth.Adapter, a bluetooth.ScanResult) {
		if a.LocalName() == name {
			select {
			case resultCh <- advertisementToBeacon(a):
			default:
			}
			stopScan()
		}
	})

	if err != nil {
		return nil, err
	}

	select {
	case r := <-resultCh:
		return r, nil
	default:
		return nil, scanCtx.Err()
	}
}

func (s *adapter) Connect(ctx context.Context, beacon *ble.Beacon) (ble.Device, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	params := bluetooth.ConnectionParams{}
	if deadline, ok := ctx.Deadline(); ok {
		params.ConnectionTimeout = bluetooth.NewDuration(time.Until(deadline))
	}

	addr, err := parseAddress(beacon.Address)
	if err != nil {
		return nil, err
	}

	type connectResult struct {
		client bluetooth.Device
		err    error
	}

	ch := make(chan connectResult, 1)
	go func() {
		client, err := s.device.Connect(addr, params)
		select {
		case ch <- connectResult{client, err}:
		default:
			slog.Warn("ble adapter: Connect returned after caller gave up",
				slog.String("address", beacon.Address), slog.Any("err", err))
			if err == nil {
				_ = client.Disconnect()
			}
		}
	}()

	select {
	case res := <-ch:
		if res.err != nil {
			return nil, res.err
		}
		return &device{client: &res.client}, nil

	case <-ctx.Done():
		select {
		case res := <-ch:
			if res.err != nil {
				return nil, res.err
			}
			return &device{client: &res.client}, nil

		case <-time.After(connectGracePeriod):
			slog.Warn("ble adapter stuck: Connect did not return after context cancellation",
				slog.String("address", beacon.Address))
			return nil, fmt.Errorf("ble: adapter stuck, connect did not return after cancel")
		}
	}
}

func (s *adapter) Close() error {
	s.device = nil
	return nil
}

func advertisementToBeacon(result bluetooth.ScanResult) *ble.Beacon {
	return &ble.Beacon{
		Address:     result.Address.String(),
		LocalName:   result.LocalName(),
		RSSI:        result.RSSI,
		Connectable: true,
	}
}

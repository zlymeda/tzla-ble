package goble

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/teslamotors/vehicle-command/pkg/protocol"
	goble "github.com/zlymeda/go-ble"
	"github.com/zlymeda/tzla-ble"
)

var ErrAdapterInvalidID = protocol.NewError("the bluetooth adapter ID is invalid", false, false)

func NewAdapter(id string) (ble.Adapter, error) {
	device, err := newAdapter(id)
	if err != nil {
		return nil, err
	}

	return &adapter{
		device: device,
	}, nil
}

type adapter struct {
	device goble.Device
}

func (s *adapter) ScanBeacon(ctx context.Context, name string) (*ble.Beacon, error) {
	scanCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var result *ble.Beacon

	fn := func(a goble.Advertisement) {
		if name != a.LocalName() {
			return
		}

		result = advertisementToBeacon(a)
		cancel()
	}

	err := s.device.Scan(scanCtx, false, fn)
	if err != nil && result == nil {
		return nil, err
	}

	return result, nil
}

func (s *adapter) Connect(ctx context.Context, beacon *ble.Beacon) (ble.Device, error) {
	type dialResult struct {
		client goble.Client
		err    error
	}

	ch := make(chan dialResult, 1)
	go func() {
		client, err := s.device.Dial(ctx, goble.NewAddr(beacon.Address))
		select {
		case ch <- dialResult{client, err}:
		default:
			// Nobody is reading — Connect already returned due to timeout.
			// Close the client if we got one to avoid resource leaks.
			slog.Warn("ble adapter: Dial returned after Connect gave up",
				slog.String("address", beacon.Address), slog.Any("err", err))
			if client != nil {
				_ = client.CancelConnection()
			}
		}
	}()

	select {
	case res := <-ch:
		if res.err != nil {
			return nil, res.err
		}
		return &device{client: res.client}, nil

	case <-ctx.Done():
		// Context expired — give Dial a grace period to finish cancelDial.
		select {
		case res := <-ch:
			if res.err != nil {
				return nil, res.err
			}
			return &device{client: res.client}, nil

		case <-time.After(5 * time.Second):
			slog.Warn("ble adapter stuck: Dial did not return after context cancellation",
				slog.String("address", beacon.Address))
			return nil, fmt.Errorf("ble: adapter stuck, dial did not return after cancel")
		}
	}
}

func (s *adapter) Close() error {
	if s.device == nil {
		return nil
	}

	device := s.device
	s.device = nil
	return device.Stop()
}

func advertisementToBeacon(a goble.Advertisement) *ble.Beacon {
	return &ble.Beacon{
		Address:     a.Addr().String(),
		LocalName:   a.LocalName(),
		RSSI:        int16(a.RSSI()),
		Connectable: a.Connectable(),
	}
}

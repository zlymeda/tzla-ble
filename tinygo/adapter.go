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

	var result *ble.Beacon
	err := s.device.Scan(func(_ *bluetooth.Adapter, a bluetooth.ScanResult) {
		if a.LocalName() == name {
			result = advertisementToBeacon(a)
			stopScan()
		}
	})

	if err != nil {
		return nil, err
	}

	if result == nil {
		return nil, scanCtx.Err()
	}

	return result, err
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

	client, err := s.device.Connect(addr, params)
	if err != nil {
		return nil, err
	}

	return &device{client: &client}, nil
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

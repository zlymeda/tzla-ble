package tinygo

import (
	"context"
	"fmt"
	"sync"

	"github.com/zlymeda/tzla-ble"
	"tinygo.org/x/bluetooth"
)

type device struct {
	client *bluetooth.Device
	mu     sync.Mutex
}

func (c *device) Service(_ context.Context, uuid string) (ble.Service, error) {
	services, err := c.client.DiscoverServices([]bluetooth.UUID{mustParseUUID(uuid)})
	if err != nil {
		return nil, fmt.Errorf("ble: failed to enumerate device services: %s", err)
	}
	if len(services) != 1 {
		return nil, fmt.Errorf("ble: failed to discover service")
	}

	return &service{client: c.client, service: services[0]}, nil
}

func (c *device) Close() error {
	c.mu.Lock()
	client := c.client
	c.client = nil
	c.mu.Unlock()
	if client == nil {
		return nil
	}
	return client.Disconnect()
}

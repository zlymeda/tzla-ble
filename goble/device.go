package goble

import (
	"context"
	"errors"
	"fmt"

	goble "github.com/zlymeda/go-ble"
	"github.com/zlymeda/tzla-ble"
)

type device struct {
	client goble.Client
}

func (c *device) Service(_ context.Context, uuid string) (ble.Service, error) {
	services, err := c.client.DiscoverServices([]goble.UUID{goble.MustParse(uuid)})
	if err != nil {
		return nil, fmt.Errorf("ble: failed to enumerate device services: %s", err)
	}
	if len(services) == 0 {
		return nil, fmt.Errorf("ble: failed to discover service")
	}

	return &service{client: c.client, service: services[0]}, nil
}

func (c *device) Close() error {
	if c.client == nil {
		return nil
	}

	client := c.client
	c.client = nil

	err1 := client.ClearSubscriptions()
	err2 := client.CancelConnection()

	return errors.Join(err1, err2)
}

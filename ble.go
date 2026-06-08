package ble

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/teslamotors/vehicle-command/pkg/connector"
	"github.com/teslamotors/vehicle-command/pkg/protocol"
)

var (
	ErrMaxConnectionsExceeded = protocol.NewError("the vehicle is already connected to the maximum number of BLE devices", false, false)

	inboxDebug = os.Getenv("BLE_INBOX_DEBUG") != ""
)

const (
	defaultMTU        = 23
	maxBLEMTUSize     = 512 + 3
	maxBLEMessageSize = 1024

	rxTimeout  = time.Second     // Timeout interval between receiving chunks of a message
	maxLatency = 4 * time.Second // Max allowed error when syncing a vehicle clock

	inboxSize = 100
)

//goland:noinspection ALL
const (
	vehicleServiceUUID = "00000211-b2d1-43f0-9b88-960cebf8b91e"
	toVehicleUUID      = "00000212-b2d1-43f0-9b88-960cebf8b91e"
	fromVehicleUUID    = "00000213-b2d1-43f0-9b88-960cebf8b91e"
)

func VehicleLocalName(vin string) string {
	vinBytes := []byte(vin)
	digest := sha1.Sum(vinBytes)
	return fmt.Sprintf("S%02xC", digest[:8])
}

type Connection struct {
	vin      string
	inbox    chan []byte
	device   Device
	writer   Writer
	cancelRx func() error

	blockLength int
	inputBuffer []byte
	lastRx      time.Time
	inboxHWM    int
	rxLock      sync.Mutex // protects inputBuffer and lastRx
	lock        sync.Mutex
	closed      bool
	closeErr    error
}

func ScanVehicleBeacon(ctx context.Context, vin string, adapter Adapter) (*Beacon, error) {
	return adapter.ScanBeacon(ctx, VehicleLocalName(vin))
}

func NewConnection(ctx context.Context, vin string, adapter Adapter) (*Connection, error) {
	beacon, err := adapter.ScanBeacon(ctx, VehicleLocalName(vin))
	if err != nil {
		return nil, err
	}
	return NewConnectionFromBeacon(ctx, vin, beacon, adapter)
}

func NewConnectionFromBeacon(ctx context.Context, vin string, beacon *Beacon, adapter Adapter) (*Connection, error) {
	var lastError error

	if beacon.LocalName != VehicleLocalName(vin) {
		return nil, fmt.Errorf("ble: beacon with unexpected local name: '%s'", beacon.LocalName)
	}

	if !beacon.Connectable {
		return nil, ErrMaxConnectionsExceeded
	}

	for {
		conn, err := tryToConnect(ctx, vin, beacon, adapter)
		if err == nil {
			return conn, nil
		}

		slog.Warn("BLE connection attempt failed", slog.Any("error", err))
		if isStaleDeviceError(err) {
			return nil, err
		}
		if err := ctx.Err(); err != nil {
			if lastError != nil {
				return nil, lastError
			}
			return nil, err
		}
		lastError = err
		time.Sleep(100 * time.Millisecond)
	}
}

// isStaleDeviceError returns true when the BlueZ D-Bus device object no longer
// exists — the car disconnected and BlueZ cleaned it up. Retrying with the same
// beacon address will keep failing; the caller must rescan.
func isStaleDeviceError(err error) bool {
	return strings.Contains(err.Error(), "doesn't exist")
}

func resolvedInboxSize() int {
	if s := os.Getenv("BLE_INBOX_SIZE"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			return n
		}
	}
	return inboxSize
}

func tryToConnect(ctx context.Context, vin string, beacon *Beacon, adapter Adapter) (result *Connection, err error) {
	device, err := adapter.Connect(ctx, beacon)
	if err != nil {
		return nil, err
	}
	defer func() {
		if result == nil {
			_ = device.Close()
		}
	}()

	service, err := device.Service(ctx, vehicleServiceUUID)
	if err != nil {
		return nil, err
	}

	writer, err := service.Tx(toVehicleUUID)
	if err != nil {
		return nil, err
	}

	txMtu, err := writer.MTU(maxBLEMTUSize)
	if err != nil {
		txMtu = defaultMTU - 3 // Fallback to default MTU size
	} else {
		txMtu = min(txMtu, maxBLEMessageSize) - 3 // 3 bytes for header
	}

	conn := &Connection{
		vin:    vin,
		inbox:  make(chan []byte, resolvedInboxSize()),
		device: device,
		writer: writer,

		blockLength: txMtu,
	}

	cancelRx, err := service.Rx(fromVehicleUUID, conn.rx)
	if err != nil {
		return nil, err
	}
	conn.cancelRx = cancelRx

	return conn, nil
}

func (c *Connection) Receive() <-chan []byte {
	return c.inbox
}

func (c *Connection) Send(ctx context.Context, buffer []byte) error {
	if len(buffer) > maxBLEMessageSize {
		return fmt.Errorf("ble: message too large: %d > %d", len(buffer), maxBLEMessageSize)
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	var out []byte
	slog.Debug("TX", "hex", hex.EncodeToString(buffer))
	out = append(out, uint8(len(buffer)>>8), uint8(len(buffer)))
	out = append(out, buffer...)
	blockLength := c.blockLength
	for len(out) > 0 {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if blockLength > len(out) {
			blockLength = len(out)
		}

		n, err := c.writer.Write(out[:blockLength])
		if err != nil {
			return err
		} else if n != blockLength {
			return fmt.Errorf("ble: failed to write %d bytes", blockLength)
		}

		out = out[blockLength:]
	}
	return nil
}

func (c *Connection) VIN() string {
	return c.vin
}

func (c *Connection) Close() {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.closed {
		return
	}
	c.closed = true

	if n := len(c.inbox); n > 0 {
		slog.Warn("BLE connection closing with unread inbox messages", slog.String("vin", c.vin), slog.Int("unread", n))
	}

	if c.cancelRx != nil {
		if err := c.cancelRx(); err != nil && !isStaleDeviceError(err) {
			slog.Warn("BLE RX unsubscribe failed", slog.String("vin", c.vin), slog.Any("error", err))
		}
		c.cancelRx = nil
	}

	if err := c.device.Close(); err != nil {
		slog.Warn("BLE connection close failed", slog.String("vin", c.vin), slog.Any("error", err))
		c.closeErr = err
	}
}

func (c *Connection) PreferredAuthMethod() connector.AuthMethod {
	return connector.AuthMethodGCM
}

func (c *Connection) RetryInterval() time.Duration {
	return time.Second
}

func (c *Connection) AllowedLatency() time.Duration {
	return maxLatency
}

func (c *Connection) CloseErr() error {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.closeErr
}

func (c *Connection) rx(p []byte) {
	c.rxLock.Lock()
	defer c.rxLock.Unlock()

	if time.Since(c.lastRx) > rxTimeout {
		if len(c.inputBuffer) > 0 {
			slog.Warn("BLE RX buffer reset: gap in BLE stream, partial message discarded",
				slog.String("vin", c.vin),
				slog.Int("discarded", len(c.inputBuffer)),
			)
		}
		c.inputBuffer = []byte{}
	}
	c.lastRx = time.Now()
	c.inputBuffer = append(c.inputBuffer, p...)
	for c.flush() {
	}
}

func (c *Connection) flush() bool {
	if len(c.inputBuffer) >= 2 {
		msgLength := 256*int(c.inputBuffer[0]) + int(c.inputBuffer[1])
		if msgLength > maxBLEMessageSize {
			slog.Warn("BLE RX oversized message, resetting buffer",
				slog.String("vin", c.vin),
				slog.Int("msgLength", msgLength),
				slog.Int("max", maxBLEMessageSize),
			)
			c.inputBuffer = []byte{}
			return false
		}
		if len(c.inputBuffer) >= 2+msgLength {
			buffer := bytes.Clone(c.inputBuffer[2 : 2+msgLength])
			slog.Debug("RX", "hex", hex.EncodeToString(buffer))
			c.inputBuffer = c.inputBuffer[2+msgLength:]
			select {
			case c.inbox <- buffer:
			default:
				// Drain oldest message to make room for the newest
				select {
				case dropped := <-c.inbox:
					slog.Warn("BLE inbox full, dropped oldest message to make room",
						slog.String("vin", c.vin),
						slog.Int("droppedLen", len(dropped)),
						slog.Int("newLen", msgLength),
						slog.Int("depth", len(c.inbox)),
						slog.Int("cap", cap(c.inbox)),
					)
				default:
				}
				c.inbox <- buffer
			}
			if inboxDebug {
				if depth := len(c.inbox); depth > c.inboxHWM {
					c.inboxHWM = depth
					slog.Debug("BLE inbox high-water mark", slog.String("vin", c.vin), slog.Int("hwm", depth), slog.Int("cap", cap(c.inbox)))
				}
			}
			return true
		}
	}
	return false
}

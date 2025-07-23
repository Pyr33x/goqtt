package er

import (
	"errors"
	"fmt"
)

type Err struct {
	Context string
	Message error
}

var (
	ErrEmptyBuffer       = errors.New("Buffer is empty")
	ErrReadBuffer        = errors.New("Could not read buffer")
	ErrShortBuffer       = errors.New("Buffer is too short for string length")
	ErrReadProtoName     = errors.New("Failed to read protocol name")
	ErrMissProtoVer      = errors.New("Missing protocol version")
	ErrMissConnectFlags  = errors.New("Missing connect flags")
	ErrMissKeepAlive     = errors.New("Missing Keep Alive")
	ErrReadClientID      = errors.New("Failed to read client ID")
	ErrInvalidConnPacket = errors.New("Connect packet is invalid")
)

func (e *Err) Error() string {
	return fmt.Sprintf("context: %s, message: %v", e.Context, e.Message)
}

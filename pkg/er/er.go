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
	ErrEmptyBuffer           = errors.New("buffer is empty")
	ErrReadBuffer            = errors.New("could not read buffer")
	ErrShortBuffer           = errors.New("buffer is too short for string length")
	ErrReadProtoName         = errors.New("failed to read protocol name")
	ErrMissProtoVer          = errors.New("missing protocol version")
	ErrMissProtoLevel        = errors.New("missing protocol level")
	ErrMissConnectFlags      = errors.New("missing connect flags")
	ErrMissKeepAlive         = errors.New("missing Keep Alive")
	ErrReadClientID          = errors.New("failed to read client ID")
	ErrInvalidConnPacket     = errors.New("connect packet is invalid")
	ErrInvalidPacketType     = errors.New("packet type is invalid")
	ErrRemainingLenMissmatch = errors.New("remaining length mismatch")
	ErrShortString           = errors.New("string is too short")
)

func (e *Err) Error() string {
	return fmt.Sprintf("context: %s, message: %v", e.Context, e.Message)
}

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
	ErrMissConnFlags         = errors.New("missing connect flags")
	ErrMissKeepAlive         = errors.New("missing Keep Alive")
	ErrReadClientID          = errors.New("failed to read client ID")
	ErrInvalidConnPacket     = errors.New("connect packet is invalid")
	ErrInvalidPacketType     = errors.New("packet type is invalid")
	ErrRemainingLenMissmatch = errors.New("remaining length mismatch")
	ErrShortString           = errors.New("string is too short")
	ErrIdentifierRejected    = errors.New("identifier rejected")

	// ClientID
	ErrEmptyClientID                = errors.New("empty client id requires clean session to be 1")
	ErrEmptyAndCleanSessionClientID = errors.New("client id is empty and clean session is set to 0")
	ErrClientIDLengthExceed         = errors.New("client id exceeds 23 bytes")
	ErrInvalidCharsClientID         = errors.New("client id contains invaid characters")

	// Protocol
	ErrUnsupportedProtocolLevel = errors.New("protocol level is not supported")
	ErrUnsupportedProtocolName  = errors.New("protocol name is not supported")

	// ConnectFlags
	ErrInvalidWillQos = errors.New("willqos level is invalid")

	// Client
	ErrClientMustSetCleanSession = errors.New("client must set clean session to 1")
)

func (e *Err) Error() string {
	return fmt.Sprintf("context: %s, message: %v", e.Context, e.Message)
}

func (e *Err) Unwrap() error {
	return e.Message
}

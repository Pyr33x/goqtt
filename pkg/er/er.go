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
	ErrShortBuffer                  = errors.New("buffer is too short")
	ErrInvalidConnPacket            = errors.New("connect packet is invalid")
	ErrInvalidPacketType            = errors.New("packet type is invalid")
	ErrIdentifierRejected           = errors.New("identifier rejected")
	ErrEmptyClientID                = errors.New("empty client id requires clean session to be 1")
	ErrEmptyAndCleanSessionClientID = errors.New("client id is empty and clean session is set to 0")
	ErrClientIDLengthExceed         = errors.New("client id exceeds 23 bytes")
	ErrInvalidCharsClientID         = errors.New("client id contains invalid characters")
	ErrUnsupportedProtocolLevel     = errors.New("protocol level is not supported")
	ErrUnsupportedProtocolName      = errors.New("protocol name is not supported")
	ErrInvalidWillQos               = errors.New("willqos level is invalid")
	ErrPasswordWithoutUsername      = errors.New("password flag set without username flag")
	ErrMalformedUsernameField       = errors.New("malformed username field")
	ErrMalformedPasswordField       = errors.New("malformed password field")
)

func (e *Err) Error() string {
	return fmt.Sprintf("context: %s, message: %v", e.Context, e.Message)
}

func (e *Err) Unwrap() error {
	return e.Message
}

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
	ErrShortBuffer                    = errors.New("buffer is too short")
	ErrInvalidConnPacket              = errors.New("connect packet is invalid")
	ErrInvalidPacketType              = errors.New("packet type is invalid")
	ErrIdentifierRejected             = errors.New("identifier rejected")
	ErrEmptyClientID                  = errors.New("empty client id requires clean session to be 1")
	ErrEmptyAndCleanSessionClientID   = errors.New("client id is empty and clean session is set to 0")
	ErrClientIDLengthExceed           = errors.New("client id exceeds 23 bytes")
	ErrInvalidCharsClientID           = errors.New("client id contains invalid characters")
	ErrUnsupportedProtocolLevel       = errors.New("protocol level is not supported")
	ErrUnsupportedProtocolName        = errors.New("protocol name is not supported")
	ErrInvalidWillQos                 = errors.New("willqos level is invalid")
	ErrPasswordWithoutUsername        = errors.New("password flag set without username flag")
	ErrMalformedUsernameField         = errors.New("malformed username field")
	ErrMalformedPasswordField         = errors.New("malformed password field")
	ErrHashFailed                     = errors.New("failed to hash password")
	ErrUserNotFound                   = errors.New("user does not exist")
	ErrInvalidPassword                = errors.New("password is invalid")
	ErrInvalidPublishPacket           = errors.New("publish packet is invalid")
	ErrPublishRemainingLengthExceeded = errors.New("remaining length exceeds maximum of 4 bytes")
	ErrInvalidQoSLevel                = errors.New("qos level is invalid")
	ErrWildcardsNotAllowedInPublish   = errors.New("wildcards not allowed in publish topic")
	ErrMissingPacketID                = errors.New("packet ID required for QoS > 0")
	ErrInvalidPacketID                = errors.New("packet ID cannot be 0")
	ErrInvalidReservedFlag            = errors.New("reserved flag bit must be 0")
	ErrInvalidPacketLength            = errors.New("packet length mismatch")
	ErrInvalidDUPFlag                 = errors.New("DUP flag cannot be set for QoS 0")
	ErrEmptyTopic                     = errors.New("topic cannot be empty")
	ErrTopicTooLong                   = errors.New("topic exceeds maximum length")
	ErrPayloadTooLarge                = errors.New("payload exceeds maximum size")
	ErrNullCharacterInTopic           = errors.New("null character not allowed in topic")
	ErrInvalidUTF8Topic               = errors.New("topic must be valid UTF-8")
	ErrControlCharacterInTopic        = errors.New("control characters not allowed in topic")
)

func (e *Err) Error() string {
	return fmt.Sprintf("context: %s, message: %v", e.Context, e.Message)
}

func (e *Err) Unwrap() error {
	return e.Message
}

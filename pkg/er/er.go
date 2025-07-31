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
	ErrInvalidDisconnectPacket        = errors.New("disconnect packet is invalid")
	ErrInvalidSubscribePacket         = errors.New("subscribe packet is invalid")
	ErrInvalidSubscribeFlags          = errors.New("subscribe fixed header flags must be 0010")
	ErrEmptyTopicFilter               = errors.New("topic filter cannot be empty")
	ErrMissingQoSByte                 = errors.New("missing QoS byte for topic filter")
	ErrInvalidQoSReservedBits         = errors.New("QoS reserved bits must be 0")
	ErrNoTopicFilters                 = errors.New("subscribe must contain at least one topic filter")
	ErrInvalidUTF8TopicFilter         = errors.New("topic filter must be valid UTF-8")
	ErrNullCharacterInTopicFilter     = errors.New("null character not allowed in topic filter")
	ErrControlCharacterInTopicFilter  = errors.New("control characters not allowed in topic filter")
	ErrMultiLevelWildcardNotLast      = errors.New("multi-level wildcard # must be the last character")
	ErrMultiLevelWildcardNotAlone     = errors.New("multi-level wildcard # must be preceded by / or be alone")
	ErrSingleLevelWildcardNotAlone    = errors.New("single-level wildcard + must be between / characters or at boundaries")
	ErrInvalidUnsubscribePacket       = errors.New("unsubscribe packet is invalid")
	ErrInvalidUnsubscribeFlags        = errors.New("unsubscribe fixed header flags must be 0010")
	ErrInvalidPingreqPacket           = errors.New("pingreq packet is invalid")
	ErrInvalidPingreqFlags            = errors.New("pingreq fixed header flags must be 0000")
	ErrInvalidPingreqLength           = errors.New("pingreq remaining length must be 0")
	ErrInvalidPingrespPacket          = errors.New("pingresp packet is invalid")
	ErrInvalidPingrespFlags           = errors.New("pingresp fixed header flags must be 0000")
	ErrInvalidPingrespLength          = errors.New("pingresp remaining length must be 0")
	ErrRemainingLengthExceeded        = errors.New("remaining length exceeds maximum of 4 bytes")
	ErrInvalidUTF8String              = errors.New("string must be valid UTF-8")
	ErrEmptyTopicLevel                = errors.New("empty topic level not allowed")
	ErrInvalidSingleLevelWildcard     = errors.New("single-level wildcard + must be alone in its level")
	ErrInvalidMultiLevelWildcard      = errors.New("multi-level wildcard # must be alone in its level")
)

func (e *Err) Error() string {
	return fmt.Sprintf("context: %s, message: %v", e.Context, e.Message)
}

func (e *Err) Unwrap() error {
	return e.Message
}

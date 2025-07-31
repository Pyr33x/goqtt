package utils

import (
	"encoding/binary"
	"unicode/utf8"

	"github.com/pyr33x/goqtt/pkg/er"
)

// EncodeRemainingLength encodes the remaining length field according to MQTT specification
// Supports up to 4 bytes (max value: 268,435,455)
func EncodeRemainingLength(length int) []byte {
	if length < 0 {
		return []byte{0}
	}

	var encoded []byte

	for {
		encodedByte := byte(length % 128)
		length = length / 128

		if length > 0 {
			encodedByte |= 128 // Set continuation bit
		}

		encoded = append(encoded, encodedByte)

		if length == 0 {
			break
		}

		// Prevent infinite loop for values that are too large
		if len(encoded) >= 4 {
			break
		}
	}

	return encoded
}

// ParseRemainingLength decodes the remaining length field from raw bytes
// Returns the decoded length, the number of bytes consumed, and any error
func ParseRemainingLength(data []byte) (int, int, error) {
	var length int
	multiplier := 1
	var offset int

	for {
		if offset >= len(data) {
			return 0, 0, &er.Err{
				Context: "ParseRemainingLength",
				Message: er.ErrShortBuffer,
			}
		}
		if offset >= 4 {
			return 0, 0, &er.Err{
				Context: "ParseRemainingLength",
				Message: er.ErrRemainingLengthExceeded,
			}
		}

		encodedByte := data[offset]
		length += int(encodedByte&0x7F) * multiplier

		// Check for overflow
		if length > 268435455 { // MQTT max remaining length
			return 0, 0, &er.Err{
				Context: "ParseRemainingLength",
				Message: er.ErrRemainingLengthExceeded,
			}
		}

		multiplier *= 128
		offset++

		if (encodedByte & 0x80) == 0 {
			break
		}
	}

	return length, offset, nil
}

// ParseString parses a UTF-8 string with 2-byte length prefix
// Returns the string, the number of bytes consumed, and any error
func ParseString(data []byte) (string, int, error) {
	if len(data) < 2 {
		return "", 0, &er.Err{
			Context: "ParseString",
			Message: er.ErrShortBuffer,
		}
	}

	length := binary.BigEndian.Uint16(data[0:2])
	if len(data) < int(2+length) {
		return "", 0, &er.Err{
			Context: "ParseString",
			Message: er.ErrShortBuffer,
		}
	}

	strBytes := data[2 : 2+length]
	str := string(strBytes)

	if !utf8.ValidString(str) {
		return "", 0, &er.Err{
			Context: "ParseString",
			Message: er.ErrInvalidUTF8String,
		}
	}

	return str, int(2 + length), nil
}

// ValidateTopicFilter validates a topic filter according to MQTT 3.1.1 rules
func ValidateTopicFilter(topicFilter string) error {
	if topicFilter == "" {
		return &er.Err{
			Context: "ValidateTopicFilter",
			Message: er.ErrEmptyTopicFilter,
		}
	}

	// Check for valid UTF-8
	if !utf8.ValidString(topicFilter) {
		return &er.Err{
			Context: "ValidateTopicFilter",
			Message: er.ErrInvalidUTF8Topic,
		}
	}

	// Check for null characters
	for _, r := range topicFilter {
		if r == 0 {
			return &er.Err{
				Context: "ValidateTopicFilter",
				Message: er.ErrNullCharacterInTopic,
			}
		}
	}

	// Check for empty levels (consecutive slashes)
	if hasEmptyLevels(topicFilter) {
		return &er.Err{
			Context: "ValidateTopicFilter",
			Message: er.ErrEmptyTopicLevel,
		}
	}

	// Validate wildcard usage
	return validateWildcards(topicFilter)
}

// ValidateTopicName validates a topic name for publishing (no wildcards allowed)
func ValidateTopicName(topicName string) error {
	if topicName == "" {
		return &er.Err{
			Context: "ValidateTopicName",
			Message: er.ErrEmptyTopic,
		}
	}

	// Check for valid UTF-8
	if !utf8.ValidString(topicName) {
		return &er.Err{
			Context: "ValidateTopicName",
			Message: er.ErrInvalidUTF8Topic,
		}
	}

	// Check for null characters
	for _, r := range topicName {
		if r == 0 {
			return &er.Err{
				Context: "ValidateTopicName",
				Message: er.ErrNullCharacterInTopic,
			}
		}
	}

	// Check for control characters (U+0001 to U+001F and U+007F to U+009F)
	for _, r := range topicName {
		if (r >= 0x0001 && r <= 0x001F) || (r >= 0x007F && r <= 0x009F) {
			return &er.Err{
				Context: "ValidateTopicName",
				Message: er.ErrControlCharacterInTopic,
			}
		}
	}

	// Topic names cannot contain wildcards
	if containsWildcards(topicName) {
		return &er.Err{
			Context: "ValidateTopicName",
			Message: er.ErrWildcardsNotAllowedInPublish,
		}
	}

	// Check for empty levels
	if hasEmptyLevels(topicName) {
		return &er.Err{
			Context: "ValidateTopicName",
			Message: er.ErrEmptyTopicLevel,
		}
	}

	return nil
}

// hasEmptyLevels checks if the topic has empty levels (consecutive slashes)
func hasEmptyLevels(topic string) bool {
	// Check for consecutive slashes
	for i := 0; i < len(topic)-1; i++ {
		if topic[i] == '/' && topic[i+1] == '/' {
			return true
		}
	}

	// Check for trailing slash which creates an empty level
	if len(topic) > 0 && topic[len(topic)-1] == '/' {
		return true
	}

	return false
}

// containsWildcards checks if a topic contains wildcard characters
func containsWildcards(topic string) bool {
	for _, char := range topic {
		if char == '+' || char == '#' {
			return true
		}
	}
	return false
}

// validateWildcards validates wildcard usage in topic filters
func validateWildcards(topicFilter string) error {
	levels := splitTopicLevels(topicFilter)

	for i, level := range levels {
		// Check single-level wildcard rules
		if containsSingleLevelWildcard(level) {
			if level != "+" {
				return &er.Err{
					Context: "ValidateTopicFilter",
					Message: er.ErrInvalidSingleLevelWildcard,
				}
			}
		}

		// Check multi-level wildcard rules
		if containsMultiLevelWildcard(level) {
			if level != "#" {
				return &er.Err{
					Context: "ValidateTopicFilter",
					Message: er.ErrInvalidMultiLevelWildcard,
				}
			}
			if i != len(levels)-1 {
				return &er.Err{
					Context: "ValidateTopicFilter",
					Message: er.ErrMultiLevelWildcardNotLast,
				}
			}
		}
	}

	return nil
}

// splitTopicLevels splits a topic into levels
func splitTopicLevels(topic string) []string {
	if topic == "" {
		return []string{}
	}

	var levels []string
	start := 0

	for i, char := range topic {
		if char == '/' {
			levels = append(levels, topic[start:i])
			start = i + 1
		}
	}

	// Add the last level
	levels = append(levels, topic[start:])

	return levels
}

// containsSingleLevelWildcard checks if a level contains the + wildcard
func containsSingleLevelWildcard(level string) bool {
	for _, char := range level {
		if char == '+' {
			return true
		}
	}
	return false
}

// containsMultiLevelWildcard checks if a level contains the # wildcard
func containsMultiLevelWildcard(level string) bool {
	for _, char := range level {
		if char == '#' {
			return true
		}
	}
	return false
}

// EncodePacketID encodes a 16-bit packet ID to bytes
func EncodePacketID(packetID uint16) []byte {
	result := make([]byte, 2)
	binary.BigEndian.PutUint16(result, packetID)
	return result
}

// ParsePacketID parses a 16-bit packet ID from bytes
func ParsePacketID(data []byte) (uint16, error) {
	if len(data) < 2 {
		return 0, &er.Err{
			Context: "ParsePacketID",
			Message: er.ErrShortBuffer,
		}
	}

	packetID := binary.BigEndian.Uint16(data[0:2])
	if packetID == 0 {
		return 0, &er.Err{
			Context: "ParsePacketID",
			Message: er.ErrInvalidPacketID,
		}
	}

	return packetID, nil
}

// CalculateFixedHeaderSize calculates the size of the fixed header
func CalculateFixedHeaderSize(remainingLength int) int {
	return 1 + len(EncodeRemainingLength(remainingLength))
}

// IsValidPacketID checks if a packet ID is valid (non-zero)
func IsValidPacketID(packetID uint16) bool {
	return packetID != 0
}

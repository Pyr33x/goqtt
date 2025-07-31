package packet

import (
	"encoding/binary"
	"unicode/utf8"

	"github.com/pyr33x/goqtt/pkg/er"
)

type UnsubscribePacket struct {
	// Fixed Header (flags are reserved and must be 0010)

	// Variable Header
	PacketID uint16

	// Payload
	TopicFilters []string

	// Raw
	Raw []byte
}

func (up *UnsubscribePacket) ParseUnsubscribe(raw []byte) error {
	if len(raw) < 2 {
		return &er.Err{
			Context: "Unsubscribe",
			Message: er.ErrInvalidUnsubscribePacket,
		}
	}

	if PacketType((raw[0] & 0xF0)) != UNSUBSCRIBE {
		return &er.Err{
			Context: "Unsubscribe",
			Message: er.ErrInvalidUnsubscribePacket,
		}
	}

	// MQTT 3.1.1: UNSUBSCRIBE fixed header flags must be 0010 (bits 3,2,1,0)
	if (raw[0] & 0x0F) != 0x02 {
		return &er.Err{
			Context: "Unsubscribe, Fixed Header",
			Message: er.ErrInvalidUnsubscribeFlags,
		}
	}

	up.Raw = raw

	// Parse remaining length to find where variable header starts
	remainingLength, offset, err := parseRemainingLength(raw[1:])
	if err != nil {
		return err
	}

	// offset is number of bytes used for remainingLength field
	// Total expected length = 1 (fixed header) + offset + remainingLength
	expectedLength := 1 + offset + remainingLength
	if len(raw) != expectedLength {
		return &er.Err{
			Context: "Unsubscribe, Packet Length",
			Message: er.ErrInvalidPacketLength,
		}
	}
	offset += 1

	// MQTT 3.1.1: UNSUBSCRIBE must have at least 4 bytes for PacketID + topic filter
	if remainingLength < 4 { // 2 bytes PacketID + 2 bytes topic length (minimum)
		return &er.Err{
			Context: "Unsubscribe",
			Message: er.ErrInvalidUnsubscribePacket,
		}
	}

	// Parse Packet ID (mandatory for UNSUBSCRIBE)
	if offset+2 > len(raw) {
		return &er.Err{
			Context: "Unsubscribe, PacketID",
			Message: er.ErrMissingPacketID,
		}
	}

	up.PacketID = binary.BigEndian.Uint16(raw[offset : offset+2])
	if up.PacketID == 0 {
		return &er.Err{
			Context: "Unsubscribe, PacketID",
			Message: er.ErrInvalidPacketID,
		}
	}
	offset += 2

	// Parse Payload (Topic Filters) - no QoS bytes unlike SUBSCRIBE
	up.TopicFilters = make([]string, 0)

	for offset < len(raw) {
		// Parse topic filter length
		if offset+2 > len(raw) {
			return &er.Err{
				Context: "Unsubscribe, Topic Filter",
				Message: er.ErrInvalidUnsubscribePacket,
			}
		}

		topicLen := binary.BigEndian.Uint16(raw[offset : offset+2])
		offset += 2

		// MQTT 3.1.1: Topic filter length validation
		if topicLen == 0 {
			return &er.Err{
				Context: "Unsubscribe, Topic Filter",
				Message: er.ErrEmptyTopicFilter,
			}
		}

		if offset+int(topicLen) > len(raw) {
			return &er.Err{
				Context: "Unsubscribe, Topic Filter",
				Message: er.ErrInvalidUnsubscribePacket,
			}
		}

		topicFilter := string(raw[offset : offset+int(topicLen)])
		offset += int(topicLen)

		// Validate topic filter
		if err := validateUnsubscribeTopicFilter(topicFilter); err != nil {
			return err
		}

		up.TopicFilters = append(up.TopicFilters, topicFilter)
	}

	// MQTT 3.1.1: UNSUBSCRIBE must contain at least one topic filter
	if len(up.TopicFilters) == 0 {
		return &er.Err{
			Context: "Unsubscribe",
			Message: er.ErrNoTopicFilters,
		}
	}

	return nil
}

func validateUnsubscribeTopicFilter(topicFilter string) error {
	// MQTT 3.1.1: Topic filter must be valid UTF-8
	if !utf8.ValidString(topicFilter) {
		return &er.Err{
			Context: "Unsubscribe, Topic Filter",
			Message: er.ErrInvalidUTF8TopicFilter,
		}
	}

	// Check for null characters (not allowed in UTF-8 strings)
	for _, char := range topicFilter {
		if char == 0 {
			return &er.Err{
				Context: "Unsubscribe, Topic Filter",
				Message: er.ErrNullCharacterInTopicFilter,
			}
		}
	}

	// Check for control characters (U+0001 to U+001F and U+007F to U+009F)
	for _, r := range topicFilter {
		if (r >= 0x0001 && r <= 0x001F) || (r >= 0x007F && r <= 0x009F) {
			return &er.Err{
				Context: "Unsubscribe, Topic Filter",
				Message: er.ErrControlCharacterInTopicFilter,
			}
		}
	}

	// Validate wildcard usage (same rules as SUBSCRIBE)
	if err := validateWildcards(topicFilter); err != nil {
		return err
	}

	return nil
}

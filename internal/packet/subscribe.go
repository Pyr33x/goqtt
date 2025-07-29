package packet

import (
	"encoding/binary"
	"unicode/utf8"

	"github.com/pyr33x/goqtt/pkg/er"
)

type SubscribeFilter struct {
	Topic string
	QoS   QoSLevel
}

type SubscribePacket struct {
	// Fixed Header (flags are reserved and must be 0010)

	// Variable Header
	PacketID uint16

	// Payload
	Filters []SubscribeFilter

	// Raw
	Raw []byte
}

func ParseSubscribe(raw []byte) (*SubscribePacket, error) {
	if len(raw) < 2 {
		return nil, &er.Err{
			Context: "Subscribe",
			Message: er.ErrInvalidSubscribePacket,
		}
	}

	if PacketType((raw[0] & 0xF0)) != SUBSCRIBE {
		return nil, &er.Err{
			Context: "Subscribe",
			Message: er.ErrInvalidSubscribePacket,
		}
	}

	// MQTT 3.1.1: SUBSCRIBE fixed header flags must be 0010 (bits 3,2,1,0)
	if (raw[0] & 0x0F) != 0x02 {
		return nil, &er.Err{
			Context: "Subscribe, Fixed Header",
			Message: er.ErrInvalidSubscribeFlags,
		}
	}

	packet := &SubscribePacket{Raw: raw}

	// Parse remaining length to find where variable header starts
	remainingLength, offset, err := parseRemainingLength(raw[1:])
	if err != nil {
		return nil, err
	}

	// offset is number of bytes used for remainingLength field
	// Total expected length = 1 (fixed header) + offset + remainingLength
	expectedLength := 1 + offset + remainingLength
	if len(raw) != expectedLength {
		return nil, &er.Err{
			Context: "Subscribe, Packet Length",
			Message: er.ErrInvalidPacketLength,
		}
	}
	offset += 1

	// MQTT 3.1.1: SUBSCRIBE must have at least 2 bytes for PacketID + topic filter
	if remainingLength < 5 { // 2 bytes PacketID + 2 bytes topic length + 1 byte topic + 1 byte QoS
		return nil, &er.Err{
			Context: "Subscribe",
			Message: er.ErrInvalidSubscribePacket,
		}
	}

	// Parse Packet ID (mandatory for SUBSCRIBE)
	if offset+2 > len(raw) {
		return nil, &er.Err{
			Context: "Subscribe, PacketID",
			Message: er.ErrMissingPacketID,
		}
	}

	packet.PacketID = binary.BigEndian.Uint16(raw[offset : offset+2])
	if packet.PacketID == 0 {
		return nil, &er.Err{
			Context: "Subscribe, PacketID",
			Message: er.ErrInvalidPacketID,
		}
	}
	offset += 2

	// Parse Payload (Topic Filters)
	packet.Filters = make([]SubscribeFilter, 0)

	for offset < len(raw) {
		// Parse topic filter length
		if offset+2 > len(raw) {
			return nil, &er.Err{
				Context: "Subscribe, Topic Filter",
				Message: er.ErrInvalidSubscribePacket,
			}
		}

		topicLen := binary.BigEndian.Uint16(raw[offset : offset+2])
		offset += 2

		// MQTT 3.1.1: Topic filter length validation
		if topicLen == 0 {
			return nil, &er.Err{
				Context: "Subscribe, Topic Filter",
				Message: er.ErrEmptyTopicFilter,
			}
		}

		if offset+int(topicLen) > len(raw) {
			return nil, &er.Err{
				Context: "Subscribe, Topic Filter",
				Message: er.ErrInvalidSubscribePacket,
			}
		}

		topicFilter := string(raw[offset : offset+int(topicLen)])
		offset += int(topicLen)

		// Validate topic filter
		if err := validateTopicFilter(topicFilter); err != nil {
			return nil, err
		}

		// Parse QoS byte
		if offset >= len(raw) {
			return nil, &er.Err{
				Context: "Subscribe, QoS",
				Message: er.ErrMissingQoSByte,
			}
		}

		qosByte := raw[offset]
		// MQTT 3.1.1: Reserved bits (7,6,5,4,3,2) must be 0
		if (qosByte & 0xFC) != 0 {
			return nil, &er.Err{
				Context: "Subscribe, QoS",
				Message: er.ErrInvalidQoSReservedBits,
			}
		}

		qos := QoSLevel(qosByte & 0x03)
		if qos > QoSExactlyOnce {
			return nil, &er.Err{
				Context: "Subscribe, QoS",
				Message: er.ErrInvalidQoSLevel,
			}
		}
		offset++

		packet.Filters = append(packet.Filters, SubscribeFilter{
			Topic: topicFilter,
			QoS:   qos,
		})
	}

	// MQTT 3.1.1: SUBSCRIBE must contain at least one topic filter
	if len(packet.Filters) == 0 {
		return nil, &er.Err{
			Context: "Subscribe",
			Message: er.ErrNoTopicFilters,
		}
	}

	return packet, nil
}

func validateTopicFilter(topicFilter string) error {
	// MQTT 3.1.1: Topic filter must be valid UTF-8
	if !utf8.ValidString(topicFilter) {
		return &er.Err{
			Context: "Subscribe, Topic Filter",
			Message: er.ErrInvalidUTF8TopicFilter,
		}
	}

	// Check for null characters (not allowed in UTF-8 strings)
	for _, char := range topicFilter {
		if char == 0 {
			return &er.Err{
				Context: "Subscribe, Topic Filter",
				Message: er.ErrNullCharacterInTopicFilter,
			}
		}
	}

	// Check for control characters (U+0001 to U+001F and U+007F to U+009F)
	for _, r := range topicFilter {
		if (r >= 0x0001 && r <= 0x001F) || (r >= 0x007F && r <= 0x009F) {
			return &er.Err{
				Context: "Subscribe, Topic Filter",
				Message: er.ErrControlCharacterInTopicFilter,
			}
		}
	}

	// Validate wildcard usage
	if err := validateWildcards(topicFilter); err != nil {
		return err
	}

	return nil
}

func validateWildcards(topicFilter string) error {
	runes := []rune(topicFilter)
	length := len(runes)

	for i, r := range runes {
		switch r {
		case '#':
			// Multi-level wildcard rules:
			// 1. Must be the last character
			// 2. Must be preceded by '/' or be the only character
			if i != length-1 {
				return &er.Err{
					Context: "Subscribe, Topic Filter Wildcard",
					Message: er.ErrMultiLevelWildcardNotLast,
				}
			}
			if i > 0 && runes[i-1] != '/' {
				return &er.Err{
					Context: "Subscribe, Topic Filter Wildcard",
					Message: er.ErrMultiLevelWildcardNotAlone,
				}
			}

		case '+':
			// Single-level wildcard rules:
			// 1. Must be between '/' or at start/end
			// 2. Cannot be adjacent to non-'/' characters
			if i > 0 && runes[i-1] != '/' {
				return &er.Err{
					Context: "Subscribe, Topic Filter Wildcard",
					Message: er.ErrSingleLevelWildcardNotAlone,
				}
			}
			if i < length-1 && runes[i+1] != '/' {
				return &er.Err{
					Context: "Subscribe, Topic Filter Wildcard",
					Message: er.ErrSingleLevelWildcardNotAlone,
				}
			}
		}
	}

	return nil
}

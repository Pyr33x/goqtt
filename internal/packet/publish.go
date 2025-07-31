package packet

import (
	"encoding/binary"
	"unicode/utf8"

	"github.com/pyr33x/goqtt/pkg/er"
)

type QoSLevel uint8

const (
	QoSAtMostOnce  QoSLevel = 0         // QoS 0
	QoSAtLeastOnce QoSLevel = 1         // QoS 1
	QoSExactlyOnce QoSLevel = 2         // QoS 2
	MaxPayloadSize          = 268435455 // 256MB - 1 (MQTT 3.1.1 max remaining length)
)

type PublishPacket struct {
	// Fixed Header
	DUP    bool
	QoS    QoSLevel
	Retain bool

	// Variable Header
	Topic    string
	PacketID *uint16 // nil for QoS 0, pointer to ID for QoS 1/2

	// Payload
	Payload []byte

	// Raw
	Raw []byte
}

func (pp *PublishPacket) Parse(raw []byte) error {
	if len(raw) < 2 {
		return &er.Err{
			Context: "Publish",
			Message: er.ErrInvalidPublishPacket,
		}
	}

	if PacketType((raw[0] & 0xF0)) != PUBLISH {
		return &er.Err{
			Context: "Publish",
			Message: er.ErrInvalidPublishPacket,
		}
	}

	pp.Raw = raw

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
			Context: "Publish, Packet Length",
			Message: er.ErrInvalidPacketLength,
		}
	}
	offset += 1

	// Extract flags from fixed header
	fixedHeader := raw[0]
	pp.DUP = (fixedHeader & 0x08) != 0
	pp.QoS = QoSLevel((fixedHeader & 0x06) >> 1)
	pp.Retain = (fixedHeader & 0x01) != 0

	// Validate QoS
	if pp.QoS > QoSExactlyOnce {
		return &er.Err{
			Context: "Publish, QoS",
			Message: er.ErrInvalidQoSLevel,
		}
	}

	// MQTT 3.1.1: DUP flag validation (should be 0 for new publishes from client)
	if pp.DUP && pp.QoS == QoSAtMostOnce {
		return &er.Err{
			Context: "Publish, DUP Flag",
			Message: er.ErrInvalidDUPFlag,
		}
	}

	// Parse topic name
	if offset+2 > len(raw) {
		return &er.Err{
			Context: "Publish",
			Message: er.ErrInvalidPublishPacket,
		}
	}

	topicLen := binary.BigEndian.Uint16(raw[offset : offset+2])
	offset += 2

	// MQTT 3.1.1: Topic length validation
	if topicLen == 0 {
		return &er.Err{
			Context: "Publish, Topic",
			Message: er.ErrEmptyTopic,
		}
	}

	if offset+int(topicLen) > len(raw) {
		return &er.Err{
			Context: "Publish, Topic",
			Message: er.ErrInvalidPublishPacket,
		}
	}

	pp.Topic = string(raw[offset : offset+int(topicLen)])
	offset += int(topicLen)

	// MQTT 3.1.1: Topic validation
	if err := validateTopic(pp.Topic); err != nil {
		return err
	}

	// Parse Packet ID (only for QoS > 0)
	if pp.QoS != QoSAtMostOnce {
		if offset+2 > len(raw) {
			return &er.Err{
				Context: "Publish, PacketID",
				Message: er.ErrMissingPacketID,
			}
		}

		packetID := binary.BigEndian.Uint16(raw[offset : offset+2])
		if packetID == 0 {
			return &er.Err{
				Context: "Publish, PacketID",
				Message: er.ErrInvalidPacketID,
			}
		}
		pp.PacketID = &packetID
		offset += 2
	}

	// Parse Payload (rest of the packet)
	if offset < len(raw) {
		payloadLen := len(raw) - offset

		// MQTT 3.1.1: Payload size validation
		if payloadLen > MaxPayloadSize {
			return &er.Err{
				Context: "Publish, Payload",
				Message: er.ErrPayloadTooLarge,
			}
		}

		pp.Payload = make([]byte, payloadLen)
		copy(pp.Payload, raw[offset:])
	}

	return nil
}

func parseRemainingLength(data []byte) (int, int, error) {
	var length int
	multiplier := 1
	var offset int

	for {
		if offset >= len(data) {
			return 0, 0, &er.Err{
				Context: "Publish, Remaining Length",
				Message: er.ErrShortBuffer,
			}
		}
		if offset >= 4 {
			return 0, 0, &er.Err{
				Context: "Publish, Remaining Length",
				Message: er.ErrPublishRemainingLengthExceeded,
			}
		}

		encodedByte := data[offset]
		length += int(encodedByte&0x7F) * multiplier
		multiplier *= 128

		offset++

		if (encodedByte & 0x80) == 0 {
			break
		}
	}

	return length, offset, nil
}

func containsWildcards(topic string) bool {
	for _, char := range topic {
		if char == '+' || char == '#' {
			return true
		}
	}
	return false
}

func validateTopic(topic string) error {
	// Check for wildcards (not allowed in PUBLISH)
	if containsWildcards(topic) {
		return &er.Err{
			Context: "Publish, Topic",
			Message: er.ErrWildcardsNotAllowedInPublish,
		}
	}

	// MQTT 3.1.1: Topic must be valid UTF-8
	if !utf8.ValidString(topic) {
		return &er.Err{
			Context: "Publish, Topic",
			Message: er.ErrInvalidUTF8Topic,
		}
	}

	// Check for null characters (not allowed in UTF-8 strings)
	for _, char := range topic {
		if char == 0 {
			return &er.Err{
				Context: "Publish, Topic",
				Message: er.ErrNullCharacterInTopic,
			}
		}
	}

	// Check for control characters (U+0001 to U+001F and U+007F to U+009F)
	for _, r := range topic {
		if (r >= 0x0001 && r <= 0x001F) || (r >= 0x007F && r <= 0x009F) {
			return &er.Err{
				Context: "Publish, Topic",
				Message: er.ErrControlCharacterInTopic,
			}
		}
	}

	return nil
}

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

func ParsePublish(raw []byte) (*PublishPacket, error) {
	if len(raw) < 2 {
		return nil, &er.Err{
			Context: "Publish",
			Message: er.ErrInvalidPublishPacket,
		}
	}

	if PacketType((raw[0] & 0xF0)) != PUBLISH {
		return nil, &er.Err{
			Context: "Publish",
			Message: er.ErrInvalidPublishPacket,
		}
	}

	packet := &PublishPacket{Raw: raw}

	// Parse remaining length to find where variable header starts
	remainingLength, offset, err := parseRemainingLength(raw[1:])
	if err != nil {
		return nil, err
	}
	offset += 1

	// Validate total packet length
	if len(raw) != 1+(offset-1)+remainingLength {
		return nil, &er.Err{
			Context: "Publish, Packet Length",
			Message: er.ErrInvalidPacketLength,
		}
	}

	// Extract flags from fixed header
	fixedHeader := raw[0]
	packet.DUP = (fixedHeader & 0x08) != 0
	packet.QoS = QoSLevel((fixedHeader & 0x06) >> 1)
	packet.Retain = (fixedHeader & 0x01) != 0

	// Validate QoS
	if packet.QoS > QoSExactlyOnce {
		return nil, &er.Err{
			Context: "Publish, QoS",
			Message: er.ErrInvalidQoSLevel,
		}
	}

	// MQTT 3.1.1: DUP flag validation (should be 0 for new publishes from client)
	if packet.DUP && packet.QoS == QoSAtMostOnce {
		return nil, &er.Err{
			Context: "Publish, DUP Flag",
			Message: er.ErrInvalidDUPFlag,
		}
	}

	// Parse topic name
	if offset+2 > len(raw) {
		return nil, &er.Err{
			Context: "Publish",
			Message: er.ErrInvalidPublishPacket,
		}
	}

	topicLen := binary.BigEndian.Uint16(raw[offset : offset+2])
	offset += 2

	// MQTT 3.1.1: Topic length validation
	if topicLen == 0 {
		return nil, &er.Err{
			Context: "Publish, Topic",
			Message: er.ErrEmptyTopic,
		}
	}

	if offset+int(topicLen) > len(raw) {
		return nil, &er.Err{
			Context: "Publish, Topic",
			Message: er.ErrInvalidPublishPacket,
		}
	}

	packet.Topic = string(raw[offset : offset+int(topicLen)])
	offset += int(topicLen)

	// MQTT 3.1.1: Topic validation
	if err := validateTopic(packet.Topic); err != nil {
		return nil, err
	}

	// Parse Packet ID (only for QoS > 0)
	if packet.QoS != QoSAtMostOnce {
		if offset+2 > len(raw) {
			return nil, &er.Err{
				Context: "Publish, PacketID",
				Message: er.ErrMissingPacketID,
			}
		}

		packetID := binary.BigEndian.Uint16(raw[offset : offset+2])
		if packetID == 0 {
			return nil, &er.Err{
				Context: "Publish, PacketID",
				Message: er.ErrInvalidPacketID,
			}
		}
		packet.PacketID = &packetID
		offset += 2
	}

	// Parse Payload (rest of the packet)
	if offset < len(raw) {
		payloadLen := len(raw) - offset

		// MQTT 3.1.1: Payload size validation
		if payloadLen > MaxPayloadSize {
			return nil, &er.Err{
				Context: "Publish, Payload",
				Message: er.ErrPayloadTooLarge,
			}
		}

		packet.Payload = make([]byte, payloadLen)
		copy(packet.Payload, raw[offset:])
	}

	return packet, nil
}

func parseRemainingLength(data []byte) (int, int, error) {
	var length int
	var multiplier int = 1
	var offset int

	for offset < len(data) {
		if offset >= 4 {
			return 0, 0, &er.Err{
				Context: "Publish, Remaining Length",
				Message: er.ErrPublishRemainingLengthExceeded,
			}
		}

		encodedByte := data[offset]
		length += int(encodedByte&0x7F) * multiplier

		if (encodedByte & 0x80) == 0 {
			break
		}

		multiplier *= 128
		offset++
	}

	return length, offset + 1, nil
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

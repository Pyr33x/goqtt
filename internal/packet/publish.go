package packet

import (
	"encoding/binary"

	"github.com/pyr33x/goqtt/internal/packet/utils"
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
	remainingLength, offset, err := utils.ParseRemainingLength(raw[1:])
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
	if err := utils.ValidateTopicName(pp.Topic); err != nil {
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

// Encode converts the PublishPacket to bytes
func (pp *PublishPacket) Encode() []byte {
	if pp == nil {
		return nil
	}

	var packet []byte

	// Fixed Header: Build the first byte
	firstByte := byte(PUBLISH)
	if pp.DUP {
		firstByte |= 0x08 // Set DUP flag (bit 3)
	}
	// Set QoS flags (bits 2-1)
	firstByte |= byte(pp.QoS) << 1
	if pp.Retain {
		firstByte |= 0x01 // Set RETAIN flag (bit 0)
	}

	// Calculate remaining length
	remainingLength := 0

	// Topic length (2 bytes) + topic string
	remainingLength += 2 + len(pp.Topic)

	// Packet ID for QoS 1 and 2
	if pp.QoS > QoSAtMostOnce {
		remainingLength += 2
	}

	// Payload
	remainingLength += len(pp.Payload)

	// Encode remaining length (supports up to 4 bytes)
	remainingLengthBytes := utils.EncodeRemainingLength(remainingLength)

	// Build the packet
	packet = append(packet, firstByte)
	packet = append(packet, remainingLengthBytes...)

	// Variable Header: Topic
	topicLengthBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(topicLengthBytes, uint16(len(pp.Topic)))
	packet = append(packet, topicLengthBytes...)
	packet = append(packet, []byte(pp.Topic)...)

	// Variable Header: Packet ID (for QoS 1 and 2)
	if pp.QoS > QoSAtMostOnce && pp.PacketID != nil {
		packetIDBytes := make([]byte, 2)
		binary.BigEndian.PutUint16(packetIDBytes, *pp.PacketID)
		packet = append(packet, packetIDBytes...)
	}

	// Payload
	packet = append(packet, pp.Payload...)

	return packet
}

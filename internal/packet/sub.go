package packet

import (
	"encoding/binary"

	"github.com/pyr33x/goqtt/pkg/er"
)

// SUBACK return codes
const (
	SubackMaxQoS0 byte = 0x00 // Maximum QoS 0
	SubackMaxQoS1 byte = 0x01 // Maximum QoS 1
	SubackMaxQoS2 byte = 0x02 // Maximum QoS 2
	SubackFailure byte = 0x80 // Failure
)

type SubackPacket struct {
	PacketID    uint16
	ReturnCodes []byte
}

// NewSubAck creates a SUBACK packet in response to a SUBSCRIBE packet
func NewSubAck(subscribePacket *SubscribePacket) *SubackPacket {
	returnCodes := make([]byte, len(subscribePacket.Filters))

	for i, filter := range subscribePacket.Filters {
		// Grant the requested QoS level (in a real implementation,
		// you might want to downgrade based on server policy)
		switch filter.QoS {
		case QoSAtMostOnce:
			returnCodes[i] = SubackMaxQoS0
		case QoSAtLeastOnce:
			returnCodes[i] = SubackMaxQoS1
		case QoSExactlyOnce:
			returnCodes[i] = SubackMaxQoS2
		default:
			returnCodes[i] = SubackFailure
		}
	}

	return &SubackPacket{
		PacketID:    subscribePacket.PacketID,
		ReturnCodes: returnCodes,
	}
}

// Encode converts the SUBACK packet to bytes
func (p *SubackPacket) Encode() []byte {
	// Calculate remaining length: 2 bytes (PacketID) + return codes length
	remainingLength := 2 + len(p.ReturnCodes)

	var packet []byte
	// Fixed header: SUBACK packet type (0x90) with reserved flags (0x00)
	packet = append(packet, 0x90)

	// Encode remaining length - use simple single-byte for SUBACK (max 255 return codes)
	if remainingLength > 127 {
		// Multi-byte encoding for larger packets
		packet = append(packet, byte(remainingLength%128)|0x80)
		packet = append(packet, byte(remainingLength/128))
	} else {
		packet = append(packet, byte(remainingLength))
	}

	// Variable header: Packet ID
	packetIDBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(packetIDBytes, p.PacketID)
	packet = append(packet, packetIDBytes...)

	// Payload: Return codes
	packet = append(packet, p.ReturnCodes...)
	return packet
}

// Parse parses a SUBACK packet from raw bytes
func (p *SubackPacket) Parse(raw []byte) error {
	if len(raw) < 4 {
		return &er.Err{Context: "SUBACK", Message: er.ErrShortBuffer}
	}

	if PacketType(raw[0]&0xF0) != SUBACK {
		return &er.Err{Context: "SUBACK", Message: er.ErrInvalidPacketType}
	}

	remainingLength := int(raw[1])
	if len(raw) != 2+remainingLength {
		return &er.Err{Context: "SUBACK", Message: er.ErrInvalidPacketLength}
	}

	p.PacketID = binary.BigEndian.Uint16(raw[2:4])
	p.ReturnCodes = make([]byte, remainingLength-2)
	copy(p.ReturnCodes, raw[4:])

	return nil
}

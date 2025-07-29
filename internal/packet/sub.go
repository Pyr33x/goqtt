package packet

import (
	"encoding/binary"
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

	// Encode remaining length (simple case, assuming < 128)
	var packet []byte

	// Fixed header: SUBACK packet type (0x90) with reserved flags (0x00)
	packet = append(packet, 0x90)

	// Remaining length (assuming < 128 for simplicity)
	packet = append(packet, byte(remainingLength))

	// Variable header: Packet ID
	packetIDBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(packetIDBytes, p.PacketID)
	packet = append(packet, packetIDBytes...)

	// Payload: Return codes
	packet = append(packet, p.ReturnCodes...)

	return packet
}

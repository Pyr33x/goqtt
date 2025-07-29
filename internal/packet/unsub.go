package packet

import (
	"encoding/binary"
)

type UnsubackPacket struct {
	PacketID uint16
}

// NewUnsubAck creates an UNSUBACK packet in response to an UNSUBSCRIBE packet
func NewUnsubAck(unsubscribePacket *UnsubscribePacket) *UnsubackPacket {
	return &UnsubackPacket{
		PacketID: unsubscribePacket.PacketID,
	}
}

// Encode converts the UNSUBACK packet to bytes
func (p *UnsubackPacket) Encode() []byte {
	// UNSUBACK has fixed remaining length of 2 (just the PacketID)
	remainingLength := 2

	var packet []byte

	// Fixed header: UNSUBACK packet type (0xB0) with reserved flags (0x00)
	packet = append(packet, 0xB0)

	// Remaining length (always 2 for UNSUBACK)
	packet = append(packet, byte(remainingLength))

	// Variable header: Packet ID
	packetIDBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(packetIDBytes, p.PacketID)
	packet = append(packet, packetIDBytes...)

	return packet
}

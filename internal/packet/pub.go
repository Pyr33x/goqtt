package packet

import "encoding/binary"

type PubackPacket struct {
	PacketID uint16
}

// NewPubAck creates a PUBACK packet in response to a PUBLISH packet with QoS 1
func NewPubAck(publishPacket *PublishPacket) *PubackPacket {
	if publishPacket.PacketID == nil {
		return nil // QoS 0 doesn't need PUBACK
	}

	return &PubackPacket{
		PacketID: *publishPacket.PacketID,
	}
}

// CreatePuback creates a PUBACK packet in response to a PUBLISH packet with QoS 1
func CreatePuback(publishPacket *PublishPacket) *PubackPacket {
	return NewPubAck(publishPacket)
}

// Publish received (QoS 2 publish received, part 1)
func NewPubRec(packetID uint16) []byte {
	return []byte{
		byte(PUBREC),          // Packet Type (PUBREC)
		0x02,                  // Remaining Length
		byte(packetID >> 8),   // MSB of Packet Identifier
		byte(packetID & 0xFF), // LSB of Packet Identifier
	}
}

// Publish release (QoS 2 publish received, part 2)
func NewPubRel(packetID uint16) []byte {
	return []byte{
		byte(PUBREL),          // Packet Type (PUBREL)
		0x02,                  // Remaining Length
		byte(packetID >> 8),   // MSB of Packet Identifier
		byte(packetID & 0xFF), // LSB of Packet Identifier
	}
}

// Publish complete (QoS 2 publish received, part 3)
func NewPubComp(packetID uint16) []byte {
	return []byte{
		byte(PUBCOMP),         // Packet Type (PUBCOMP)
		0x02,                  // Remaining Length
		byte(packetID >> 8),   // MSB of Packet Identifier
		byte(packetID & 0xFF), // LSB of Packet Identifier
	}
}

// Encode converts the PUBACK packet to bytes
func (p *PubackPacket) Encode() []byte {
	// PUBACK has fixed remaining length of 2 (just the PacketID)
	remainingLength := 2

	var packet []byte

	// Fixed header: PUBACK packet type (0x40) with reserved flags (0x00)
	packet = append(packet, 0x40)

	// Remaining length (always 2 for PUBACK)
	packet = append(packet, byte(remainingLength))

	// Variable header: Packet ID
	packetIDBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(packetIDBytes, p.PacketID)
	packet = append(packet, packetIDBytes...)

	return packet
}

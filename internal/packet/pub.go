package packet

// Publish Acknowledge
func NewPubAck(packetID uint16) []byte {
	return []byte{
		byte(PUBACK),          // Packet Type (PUBACK)
		0x02,                  // Remaining Length
		byte(packetID >> 8),   // MSB of Packet Identifier
		byte(packetID & 0xFF), // LSB of Packet Identifier
	}
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

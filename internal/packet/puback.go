package packet

func NewPubAck(packetID uint16) []byte {
	return []byte{
		0x40,                  // Packet Type (PUBACK) + Flags
		0x02,                  // Remaining Length
		byte(packetID >> 8),   // MSB of Packet Identifier
		byte(packetID & 0xFF), // LSB of Packet Identifier
	}
}

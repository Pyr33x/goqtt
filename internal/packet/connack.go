package packet

func NewConnAck() []byte {
	// 0x20: Packet type = CONNACK
	// 0x02: Remaining length = 2 bytes
	// 0x00: Session present = 0
	// 0x00: Return code = 0 (OK)
	return []byte{0x20, 0x02, 0x00, 0x00}
}

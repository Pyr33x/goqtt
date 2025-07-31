package packet

import (
	"encoding/binary"

	"github.com/pyr33x/goqtt/pkg/er"
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

// Parse parses an UNSUBACK packet from raw bytes
func (p *UnsubackPacket) Parse(raw []byte) error {
	if len(raw) < 4 {
		return &er.Err{Context: "UNSUBACK", Message: er.ErrShortBuffer}
	}

	if PacketType(raw[0]&0xF0) != UNSUBACK {
		return &er.Err{Context: "UNSUBACK", Message: er.ErrInvalidPacketType}
	}

	if raw[1] != 0x02 { // Remaining length must be 2
		return &er.Err{Context: "UNSUBACK", Message: er.ErrInvalidPacketLength}
	}

	p.PacketID = binary.BigEndian.Uint16(raw[2:4])
	return nil
}

// Encode converts the UNSUBACK packet to bytes
func (p *UnsubackPacket) Encode() []byte {
	// UNSUBACK has fixed remaining length of 2 (just the PacketID)
	remainingLength := 2

	var packet []byte

	// Fixed header: UNSUBACK packet type (0xB0) with reserved flags (0x00)
	packet = append(packet, byte(UNSUBACK))

	// Remaining length (always 2 for UNSUBACK)
	packet = append(packet, byte(remainingLength))

	// Variable header: Packet ID
	packetIDBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(packetIDBytes, p.PacketID)
	packet = append(packet, packetIDBytes...)

	return packet
}

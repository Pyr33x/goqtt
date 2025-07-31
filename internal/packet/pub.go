package packet

import (
	"encoding/binary"

	"github.com/pyr33x/goqtt/pkg/er"
)

type PubackPacket struct {
	PacketID uint16
}

type PubrecPacket struct {
	PacketID uint16
}

type PubrelPacket struct {
	PacketID uint16
}

type PubcompPacket struct {
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

// NewPubRec creates a PUBREC packet for QoS 2 flow
func NewPubRec(packetID uint16) *PubrecPacket {
	return &PubrecPacket{PacketID: packetID}
}

// NewPubRel creates a PUBREL packet for QoS 2 flow
func NewPubRel(packetID uint16) *PubrelPacket {
	return &PubrelPacket{PacketID: packetID}
}

// NewPubComp creates a PUBCOMP packet for QoS 2 flow
func NewPubComp(packetID uint16) *PubcompPacket {
	return &PubcompPacket{PacketID: packetID}
}

// Parse methods for QoS flow control packets
func (p *PubackPacket) Parse(raw []byte) error {
	if len(raw) < 4 {
		return &er.Err{Context: "PUBACK", Message: er.ErrShortBuffer}
	}

	if PacketType(raw[0]&0xF0) != PUBACK {
		return &er.Err{Context: "PUBACK", Message: er.ErrInvalidPacketType}
	}

	if raw[1] != 0x02 { // Remaining length must be 2
		return &er.Err{Context: "PUBACK", Message: er.ErrInvalidPacketLength}
	}

	p.PacketID = binary.BigEndian.Uint16(raw[2:4])
	return nil
}

func (p *PubrecPacket) Parse(raw []byte) error {
	if len(raw) < 4 {
		return &er.Err{Context: "PUBREC", Message: er.ErrShortBuffer}
	}

	if PacketType(raw[0]&0xF0) != PUBREC {
		return &er.Err{Context: "PUBREC", Message: er.ErrInvalidPacketType}
	}

	if raw[1] != 0x02 {
		return &er.Err{Context: "PUBREC", Message: er.ErrInvalidPacketLength}
	}

	p.PacketID = binary.BigEndian.Uint16(raw[2:4])
	return nil
}

func (p *PubrelPacket) Parse(raw []byte) error {
	if len(raw) < 4 {
		return &er.Err{Context: "PUBREL", Message: er.ErrShortBuffer}
	}

	if PacketType(raw[0]&0xF0) != PUBREL {
		return &er.Err{Context: "PUBREL", Message: er.ErrInvalidPacketType}
	}

	// PUBREL fixed header flags must be 0010
	if (raw[0] & 0x0F) != 0x02 {
		return &er.Err{Context: "PUBREL", Message: er.ErrInvalidPacketType}
	}

	if raw[1] != 0x02 {
		return &er.Err{Context: "PUBREL", Message: er.ErrInvalidPacketLength}
	}

	p.PacketID = binary.BigEndian.Uint16(raw[2:4])
	return nil
}

func (p *PubcompPacket) Parse(raw []byte) error {
	if len(raw) < 4 {
		return &er.Err{Context: "PUBCOMP", Message: er.ErrShortBuffer}
	}

	if PacketType(raw[0]&0xF0) != PUBCOMP {
		return &er.Err{Context: "PUBCOMP", Message: er.ErrInvalidPacketType}
	}

	if raw[1] != 0x02 {
		return &er.Err{Context: "PUBCOMP", Message: er.ErrInvalidPacketLength}
	}

	p.PacketID = binary.BigEndian.Uint16(raw[2:4])
	return nil
}

// Encode methods
func (p *PubackPacket) Encode() []byte {
	packet := make([]byte, 4)
	packet[0] = byte(PUBACK)
	packet[1] = 0x02
	binary.BigEndian.PutUint16(packet[2:4], p.PacketID)
	return packet
}

func (p *PubrecPacket) Encode() []byte {
	packet := make([]byte, 4)
	packet[0] = byte(PUBREC)
	packet[1] = 0x02
	binary.BigEndian.PutUint16(packet[2:4], p.PacketID)
	return packet
}

func (p *PubrelPacket) Encode() []byte {
	packet := make([]byte, 4)
	packet[0] = byte(PUBREL) | 0x02 // PUBREL requires flags 0010
	packet[1] = 0x02
	binary.BigEndian.PutUint16(packet[2:4], p.PacketID)
	return packet
}

func (p *PubcompPacket) Encode() []byte {
	packet := make([]byte, 4)
	packet[0] = byte(PUBCOMP)
	packet[1] = 0x02
	binary.BigEndian.PutUint16(packet[2:4], p.PacketID)
	return packet
}

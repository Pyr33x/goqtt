package packet

import (
	"github.com/pyr33x/goqtt/pkg/er"
)

type PingreqPacket struct {
	// PINGREQ has no variable header or payload
	Raw []byte
}

type PingrespPacket struct{}

func (pp *PingreqPacket) ParsePingreq(raw []byte) error {
	if len(raw) < 2 {
		return &er.Err{
			Context: "Pingreq",
			Message: er.ErrInvalidPingreqPacket,
		}
	}

	pp.Raw = raw

	if PacketType((raw[0] & 0xF0)) != PINGREQ {
		return &er.Err{
			Context: "Pingreq",
			Message: er.ErrInvalidPingreqPacket,
		}
	}

	// MQTT 3.1.1: PINGREQ fixed header flags must be 0000 (bits 3,2,1,0)
	if (raw[0] & 0x0F) != 0x00 {
		return &er.Err{
			Context: "Pingreq, Fixed Header",
			Message: er.ErrInvalidPingreqFlags,
		}
	}

	// MQTT 3.1.1: PINGREQ remaining length must be 0
	if raw[1] != 0x00 {
		return &er.Err{
			Context: "Pingreq, Remaining Length",
			Message: er.ErrInvalidPingreqLength,
		}
	}

	// MQTT 3.1.1: PINGREQ packet must be exactly 2 bytes
	if len(raw) != 2 {
		return &er.Err{
			Context: "Pingreq, Packet Length",
			Message: er.ErrInvalidPacketLength,
		}
	}

	return nil
}

func (pp *PingrespPacket) ParsePingresp(raw []byte) error {
	if len(raw) < 2 {
		return &er.Err{
			Context: "Pingresp",
			Message: er.ErrInvalidPingrespPacket,
		}
	}

	if PacketType((raw[0] & 0xF0)) != PINGRESP {
		return &er.Err{
			Context: "Pingresp",
			Message: er.ErrInvalidPingrespPacket,
		}
	}

	// MQTT 3.1.1: PINGRESP fixed header flags must be 0000 (bits 3,2,1,0)
	if (raw[0] & 0x0F) != 0x00 {
		return &er.Err{
			Context: "Pingresp, Fixed Header",
			Message: er.ErrInvalidPingrespFlags,
		}
	}

	// MQTT 3.1.1: PINGRESP remaining length must be 0
	if raw[1] != 0x00 {
		return &er.Err{
			Context: "Pingresp, Remaining Length",
			Message: er.ErrInvalidPingrespLength,
		}
	}

	// MQTT 3.1.1: PINGRESP packet must be exactly 2 bytes
	if len(raw) != 2 {
		return &er.Err{
			Context: "Pingresp, Packet Length",
			Message: er.ErrInvalidPacketLength,
		}
	}

	return nil
}

// CreatePingresp creates a PINGRESP packet in response to a PINGREQ packet
func CreatePingresp() *PingrespPacket {
	return &PingrespPacket{}
}

// Encode converts the PINGRESP packet to bytes
func (p *PingrespPacket) Encode() []byte {
	// PINGRESP is exactly 2 bytes: 0xD0 0x00
	return []byte{0xD0, 0x00}
}

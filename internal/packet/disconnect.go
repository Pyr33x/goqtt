package packet

import "github.com/pyr33x/goqtt/pkg/er"

type DisconnectPacket struct{}

func (dp *DisconnectPacket) Parse(raw []byte) error {
	if len(raw) < 2 {
		return &er.Err{
			Context: "Disconnect",
			Message: er.ErrInvalidDisconnectPacket,
		}
	}

	// First byte should be 0xE0 (type = 14 << 4, flags = 0)
	if PacketType(raw[0]) != DISCONNECT {
		return &er.Err{
			Context: "Disconnect, Control",
			Message: er.ErrInvalidDisconnectPacket,
		}
	}

	// Remaining length must be 0
	if raw[1] != 0x00 {
		return &er.Err{
			Context: "Disconnect, Remaining Length",
			Message: er.ErrInvalidDisconnectPacket,
		}
	}

	return nil
}

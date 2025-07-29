package packet

import "github.com/pyr33x/goqtt/pkg/er"

type DisconnectPacket struct{}

func ParseDisconnect(raw []byte) (*DisconnectPacket, error) {
	if len(raw) < 2 {
		return nil, &er.Err{
			Context: "Disconnect",
			Message: er.ErrInvalidDisconnectPacket,
		}
	}

	// First byte should be 0xE0 (type = 14 << 4, flags = 0)
	if PacketType(raw[0]) != DISCONNECT {
		return nil, &er.Err{
			Context: "Disconnect, Control",
			Message: er.ErrInvalidDisconnectPacket,
		}
	}

	// Remaining length must be 0
	if raw[1] != 0x00 {
		return nil, &er.Err{
			Context: "Disconnect, Remaining Length",
			Message: er.ErrInvalidDisconnectPacket,
		}
	}

	return &DisconnectPacket{}, nil
}

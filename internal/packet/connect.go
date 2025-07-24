package packet

import (
	"encoding/binary"

	"github.com/pyr33x/goqtt/pkg/er"
)

type ConnectPacket struct {
	// Variable Header
	ProtocolName  string
	ProtocolLevel byte
	UsernameFlag  bool
	PasswordFlag  bool
	WillRetain    bool
	WillQos       byte
	WillFlag      bool
	CleanStart    bool
	KeepAlive     uint16

	// Payload
	ClientID    string
	WillTopic   string // (if Will flag is set)
	WillMessage string // (if Will flag is set)
	Username    string // (if Username flag is set)
	Password    string // (if Password flag is set)

	// Raw
	Raw []byte
}

func ParseConnect(raw []byte) (*ConnectPacket, error) {
	if len(raw) < 10 {
		return nil, &er.Err{
			Context: "Connect",
			Message: er.ErrInvalidConnPacket,
		}
	}

	if PacketType((raw[0] & 0xF0)) != CONNECT {
		return nil, &er.Err{
			Context: "Connect",
			Message: er.ErrInvalidConnPacket,
		}
	}

	packet := &ConnectPacket{Raw: raw}
	offset := 2 // Skip fixed header (packet type + remaining length)

	if offset+2 > len(raw) {
		return nil, &er.Err{
			Context: "Connect",
			Message: er.ErrInvalidConnPacket,
		}
	}

	// Protocol Name Length (skip fixed header + 2) = Protocol
	protoNameLen := binary.BigEndian.Uint16(raw[offset : offset+2])
	offset += 2

	if offset+int(protoNameLen) > len(raw) {
		return nil, &er.Err{
			Context: "Connect",
			Message: er.ErrInvalidConnPacket,
		}
	}

	packet.ProtocolName = string(raw[offset : offset+int(protoNameLen)])
	offset += int(protoNameLen)

	// Parse Protocol Level (strict to 4 = MQTT 3.1.1)
	if offset >= len(raw) {
		return nil, &er.Err{
			Context: "Connect",
			Message: er.ErrInvalidConnPacket,
		}
	}
	packet.ProtocolLevel = raw[offset]
	offset++

	// Parse Connect Flags
	if offset >= len(raw) {
		return nil, &er.Err{
			Context: "Connect",
			Message: er.ErrInvalidConnPacket,
		}
	}
	connectFlags := raw[offset]
	offset++

	packet.UsernameFlag = (connectFlags & 0x80) != 0 // bit 7
	packet.PasswordFlag = (connectFlags & 0x40) != 0 // bit 6
	packet.WillRetain = (connectFlags & 0x20) != 0   // bit 5
	packet.WillQos = (connectFlags & 0x18) >> 3      // bit 4-3
	packet.WillFlag = (connectFlags & 0x04) != 0     // bit 2
	packet.CleanStart = (connectFlags & 0x02) != 0   // bit 1

	// Parse Keep Alive
	if offset+2 > len(raw) {
		return nil, &er.Err{
			Context: "Connect",
			Message: er.ErrInvalidConnPacket,
		}
	}
	packet.KeepAlive = binary.BigEndian.Uint16(raw[offset : offset+2])
	offset += 2

	return packet, nil
}

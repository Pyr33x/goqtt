package packet

import "github.com/pyr33x/goqtt/pkg/er"

// Parse determines the packet type and returns the appropriate parsed packet
func Parse(raw []byte) (*ParsedPacket, error) {
	if len(raw) < 1 {
		return nil, &er.Err{
			Context: "Parse",
			Message: er.ErrShortBuffer,
		}
	}

	packetType := Type(raw[0])

	result := &ParsedPacket{
		Type: packetType,
		Raw:  raw,
	}

	switch packetType {
	case CONNECT:
		connectPacket, err := ParseConnect(raw)
		if err != nil {
			return nil, err
		}
		result.Connect = connectPacket
		return result, nil

	default:
		return nil, &er.Err{
			Context: "Parse",
			Message: er.ErrInvalidPacketType,
		}
	}
}

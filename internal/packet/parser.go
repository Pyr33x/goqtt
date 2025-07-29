package packet

import "github.com/pyr33x/goqtt/pkg/er"

// Parse determines the packet type and returns the appropriate parsed packet
func Parse(raw []byte) (*ParsedPacket, error) {
	if len(raw) < 1 {
		return nil, &er.Err{
			Context: "Parser",
			Message: er.ErrShortBuffer,
		}
	}

	packetType := PacketType(raw[0])

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
	case PUBLISH:
		publishPacket, err := ParsePublish(raw)
		if err != nil {
			return nil, err
		}
		result.Publish = publishPacket
		return result, nil

	default:
		return nil, &er.Err{
			Context: "Parser",
			Message: er.ErrInvalidPacketType,
		}
	}
}

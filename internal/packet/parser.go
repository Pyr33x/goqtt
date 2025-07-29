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

	packetType := PacketType(raw[0] & 0xF0)
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

	case SUBSCRIBE:
		subscribePacket, err := ParseSubscribe(raw)
		if err != nil {
			return nil, err
		}
		result.Subscribe = subscribePacket
		return result, nil

	case UNSUBSCRIBE:
		unsubscribePacket, err := ParseUnsubscribe(raw)
		if err != nil {
			return nil, err
		}
		result.Unsubscribe = unsubscribePacket
		return result, nil

	case PINGREQ:
		pingreqPacket, err := ParsePingreq(raw)
		if err != nil {
			return nil, err
		}
		result.Pingreq = pingreqPacket
		return result, nil

	case DISCONNECT:
		disconnectPacket, err := ParseDisconnect(raw)
		if err != nil {
			return nil, err
		}
		result.Disconnect = disconnectPacket
		return result, nil

	default:
		return nil, &er.Err{
			Context: "Parser",
			Message: er.ErrInvalidPacketType,
		}
	}
}

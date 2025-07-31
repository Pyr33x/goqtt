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
		pkt := &ConnectPacket{}
		if err := pkt.Parse(raw); err != nil {
			return nil, err
		}
		result.Connect = pkt
		return result, nil

	case PUBLISH:
		pkt := &PublishPacket{}
		if err := pkt.Parse(raw); err != nil {
			return nil, err
		}
		result.Publish = pkt
		return result, nil

	case PUBACK:
		pkt := &PubackPacket{}
		if err := pkt.Parse(raw); err != nil {
			return nil, err
		}
		result.Puback = pkt
		return result, nil

	case PUBREC:
		pkt := &PubrecPacket{}
		if err := pkt.Parse(raw); err != nil {
			return nil, err
		}
		result.Pubrec = pkt
		return result, nil

	case PUBREL:
		pkt := &PubrelPacket{}
		if err := pkt.Parse(raw); err != nil {
			return nil, err
		}
		result.Pubrel = pkt
		return result, nil

	case PUBCOMP:
		pkt := &PubcompPacket{}
		if err := pkt.Parse(raw); err != nil {
			return nil, err
		}
		result.Pubcomp = pkt
		return result, nil

	case SUBSCRIBE:
		pkt := &SubscribePacket{}
		if err := pkt.Parse(raw); err != nil {
			return nil, err
		}
		result.Subscribe = pkt
		return result, nil

	case SUBACK:
		pkt := &SubackPacket{}
		if err := pkt.Parse(raw); err != nil {
			return nil, err
		}
		result.Suback = pkt
		return result, nil

	case UNSUBSCRIBE:
		pkt := &UnsubscribePacket{}
		if err := pkt.Parse(raw); err != nil {
			return nil, err
		}
		result.Unsubscribe = pkt
		return result, nil

	case UNSUBACK:
		pkt := &UnsubackPacket{}
		if err := pkt.Parse(raw); err != nil {
			return nil, err
		}
		result.Unsuback = pkt
		return result, nil

	case PINGREQ:
		pkt := &PingreqPacket{}
		if err := pkt.Parse(raw); err != nil {
			return nil, err
		}
		result.Pingreq = pkt
		return result, nil

	case DISCONNECT:
		pkt := &DisconnectPacket{}
		if err := pkt.Parse(raw); err != nil {
			return nil, err
		}
		result.Disconnect = pkt
		return result, nil

	default:
		return nil, &er.Err{
			Context: "Parser",
			Message: er.ErrInvalidPacketType,
		}
	}
}

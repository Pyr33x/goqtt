package packet

import "github.com/pyr33x/goqtt/utilities/er"

func Parse(buf []byte) (*Packet, error) {
	if len(buf) < 1 {
		return nil, &er.Err{
			Context: "Parser",
			Message: er.ErrEmptyBuffer,
		}
	}

	packetType := Type(buf[0] >> 4)
	return &Packet{
		Type: packetType,
		Raw:  buf,
	}, nil
}

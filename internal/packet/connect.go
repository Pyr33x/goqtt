package packet

import (
	"encoding/binary"

	"github.com/pyr33x/goqtt/pkg/er"
)

type ConnectPacket struct {
	ProtocolName  string
	ProtocolLevel byte
	CleanStart    bool
	KeepAlive     uint16
	ClientID      string
	Raw           []byte
}

func ParseConnect(raw []byte) (*ConnectPacket, error) {
	if len(raw) < 2 {
		return nil, &er.Err{
			Context: "Connect",
			Message: er.ErrInvalidConnPacket,
		}
	}

	if raw[0] != byte(CONNECT) {
		return nil, &er.Err{
			Context: "Connect",
			Message: er.ErrInvalidPacketType,
		}
	}

	remainingLength := int(raw[1])
	if remainingLength != len(raw)-2 {
		return nil, &er.Err{
			Context: "Connect",
			Message: er.ErrRemainingLenMissmatch,
		}
	}

	index := 2

	protoName, n, err := DecodeString(raw[index:])
	if err != nil {
		return nil, err
	}
	index += n

	if index >= len(raw) {
		return nil, &er.Err{
			Context: "Connect",
			Message: er.ErrMissProtoLevel,
		}
	}
	protoLevel := raw[index]
	index++

	if index >= len(raw) {
		return nil, &er.Err{
			Context: "Connect",
			Message: er.ErrMissConnectFlags,
		}
	}
	flags := raw[index]
	cleanStart := (flags & 0x02) > 0
	index++

	if index+2 > len(raw) {
		return nil, &er.Err{
			Context: "Connect",
			Message: er.ErrMissKeepAlive,
		}
	}
	keepAlive := binary.BigEndian.Uint16(raw[index : index+2])
	index += 2

	clientId, n, err := DecodeString(raw[index:])
	if err != nil {
		return nil, &er.Err{
			Context: "Connect",
			Message: err,
		}
	}
	index += n

	return &ConnectPacket{
		ProtocolName:  protoName,
		ProtocolLevel: protoLevel,
		CleanStart:    cleanStart,
		KeepAlive:     keepAlive,
		ClientID:      clientId,
		Raw:           raw,
	}, nil
}

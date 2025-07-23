package packet

import (
	"encoding/binary"

	"github.com/pyr33x/goqtt/utilities/er"
)

func DecodeString(b []byte) (string, int, error) {
	if len(b) < 2 {
		return "", 0, &er.Err{
			Context: "Decode",
			Message: er.ErrShortString,
		}
	}

	length := int(binary.BigEndian.Uint16(b[:2]))
	if len(b) < 2+length {
		return "", 0, &er.Err{
			Context: "Decode",
			Message: er.ErrRemainingLenMissmatch,
		}
	}

	return string(b[2 : 2+length]), 2 + length, nil
}

package packet

type Type byte

const (
	CONNECT Type = 0x10
)

type Packet struct {
	Type Type
	Raw  []byte
}

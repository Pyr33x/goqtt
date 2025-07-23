package packet

type Type byte

const (
	CONNECT   Type = 1
	CONNACK   Type = 2
	PUBLISH   Type = 3
	SUBSCRIBE Type = 8
)

type Packet struct {
	Type Type
	Raw  []byte
}

package packet

type Type byte

const (
	CONNECT Type = 0x10
)

type Packet struct {
	Type Type
	Raw  []byte
}

type ConnectPacket struct {
	ProtocolName  string
	ProtocolLevel byte
	CleanSession  bool
	KeepAlive     uint16
	ClientID      string
	Raw           []byte
}

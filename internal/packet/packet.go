package packet

// MQTT Packet Type
type PacketType byte

const (
	CONNECT     PacketType = 0x10 // Client to Server - Connection request
	CONNACK     PacketType = 0x20 // Server to Client - Connection acknowledgment
	PUBLISH     PacketType = 0x30 // Bidirectional - Publish message
	PUBACK      PacketType = 0x40 // Bidirectional - Publish acknowledgment
	PUBREC      PacketType = 0x50 // Bidirectional - Publish received
	PUBREL      PacketType = 0x60 // Bidirectional - Publish release
	PUBCOMP     PacketType = 0x70 // Bidirectional - Publish complete
	SUBSCRIBE   PacketType = 0x80 // Client to Server - Subscribe request
	SUBACK      PacketType = 0x90 // Server to Client - Subscribe acknowledgment
	UNSUBSCRIBE PacketType = 0xA0 // Client to Server - Unsubscribe request
	UNSUBACK    PacketType = 0xB0 // Server to Client - Unsubscribe acknowledgment
	PINGREQ     PacketType = 0xC0 // Client to Server - PING request
	PINGRESP    PacketType = 0xD0 // Server to Client - PING response
	DISCONNECT  PacketType = 0xE0 // Client to Server - Disconnect notification
)

type ParsedPacket struct {
	Type    PacketType
	Raw     []byte
	Connect *ConnectPacket
	Publish *PublishPacket
}

// IsConnect returns true if this is a CONNECT packet
func (p *ParsedPacket) IsConnect() bool {
	return p.Type == CONNECT && p.Connect != nil
}

// GetConnect safely returns the CONNECT packet data
func (p *ParsedPacket) GetConnect() *ConnectPacket {
	if p.IsConnect() {
		return p.Connect
	}
	return nil
}

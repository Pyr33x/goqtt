package packet

// MQTT Packet Types
type Type byte

const (
	CONNECT     Type = 0x10 // Client to Server - Connection request
	CONNACK     Type = 0x20 // Server to Client - Connection acknowledgment
	PUBLISH     Type = 0x30 // Client to Server or Server to Client - Publish message
	PUBACK      Type = 0x40 // Client to Server or Server to Client - Publish acknowledgment
	PUBREC      Type = 0x50 // Client to Server or Server to Client - Publish received
	PUBREL      Type = 0x60 // Client to Server or Server to Client - Publish release
	PUBCOMP     Type = 0x70 // Client to Server or Server to Client - Publish complete
	SUBSCRIBE   Type = 0x80 // Client to Server - Subscribe request
	SUBACK      Type = 0x90 // Server to Client - Subscribe acknowledgment
	UNSUBSCRIBE Type = 0xA0 // Client to Server - Unsubscribe request
	UNSUBACK    Type = 0xB0 // Server to Client - Unsubscribe acknowledgment
	PINGREQ     Type = 0xC0 // Client to Server - PING request
	PINGRESP    Type = 0xD0 // Server to Client - PING response
	DISCONNECT  Type = 0xE0 // Client to Server - Disconnect notification
)

type ParsedPacket struct {
	Type    Type
	Raw     []byte
	Connect *ConnectPacket
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

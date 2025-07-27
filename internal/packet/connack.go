package packet

const (
	ConnectionAccepted          = 0x00 // Connection Accepted
	UnacceptableProtocolVersion = 0x01 // The Server does not support the level of the MQTT protocol requested by the Client
	IdentifierRejected          = 0x02 // The Client identifier is correct UTF-8 but not allowed by the Server
	ServerUnavailable           = 0x03 // The Network Connection has been made but the MQTT service is unavailable
	BadUsernameOrPassword       = 0x04 // The data in the user name or password is malformed
	NotAuthorized               = 0x05 // The Client is not authorized to connect
)

func NewConnAck(sessionPresent bool, returnCode byte) []byte {
	flags := byte(0x00)
	if sessionPresent {
		flags = 0x01
	}

	return []byte{
		0x20, // Packet Type (CONNACK) + flags
		0x02, // Remaining Length (always 2)
		flags,
		returnCode,
	}
}

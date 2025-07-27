package packet

import (
	"encoding/binary"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/pyr33x/goqtt/pkg/er"
)

type ConnectPacket struct {
	// Variable Header
	ProtocolName  string
	ProtocolLevel byte
	UsernameFlag  bool
	PasswordFlag  bool
	WillRetain    bool
	WillQos       byte
	WillFlag      bool
	CleanSession  bool
	KeepAlive     uint16

	// Payload
	ClientID    string
	WillTopic   string // (if Will flag is set)
	WillMessage string // (if Will flag is set)
	Username    string // (if Username flag is set)
	Password    string // (if Password flag is set)

	// Raw
	Raw []byte
}

func ParseConnect(raw []byte) (*ConnectPacket, error) {
	if len(raw) < 10 {
		return nil, &er.Err{
			Context: "Connect",
			Message: er.ErrInvalidConnPacket,
		}
	}

	if PacketType((raw[0] & 0xF0)) != CONNECT {
		return nil, &er.Err{
			Context: "Connect",
			Message: er.ErrInvalidConnPacket,
		}
	}

	packet := &ConnectPacket{Raw: raw}
	offset := 2 // Skip fixed header (packet type + remaining length)

	if offset+2 > len(raw) {
		return nil, &er.Err{
			Context: "Connect",
			Message: er.ErrInvalidConnPacket,
		}
	}

	// Protocol Name Length (skip fixed header + 2) = Protocol
	protocolNameLen := binary.BigEndian.Uint16(raw[offset : offset+2])
	offset += 2

	if offset+int(protocolNameLen) > len(raw) {
		return nil, &er.Err{
			Context: "Connect",
			Message: er.ErrInvalidConnPacket,
		}
	}

	packet.ProtocolName = string(raw[offset : offset+int(protocolNameLen)])
	offset += int(protocolNameLen)

	// Enforce "MQTT" as ProtocolName (strict, case-sensitive)
	if packet.ProtocolName != "MQTT" {
		return nil, &er.Err{
			Context: "Connect, ProtocolName",
			Message: er.ErrUnsupportedProtocolName,
		}
	}

	// Parse Protocol Level (strict to 4 = MQTT 3.1.1)
	if offset >= len(raw) {
		return nil, &er.Err{
			Context: "Connect",
			Message: er.ErrInvalidConnPacket,
		}
	}
	packet.ProtocolLevel = raw[offset]
	offset++
	if packet.ProtocolLevel != 4 {
		return nil, &er.Err{
			Context: "Connect, ProtocolLevel",
			Message: er.ErrUnsupportedProtocolLevel,
		}
	}

	// Parse Connect Flags
	if offset >= len(raw) {
		return nil, &er.Err{
			Context: "Connect",
			Message: er.ErrInvalidConnPacket,
		}
	}
	connectFlags := raw[offset]
	offset++

	packet.UsernameFlag = (connectFlags & 0x80) != 0 // bit 7
	packet.PasswordFlag = (connectFlags & 0x40) != 0 // bit 6
	packet.WillRetain = (connectFlags & 0x20) != 0   // bit 5
	packet.WillQos = (connectFlags & 0x18) >> 3      // bit 4-3
	packet.WillFlag = (connectFlags & 0x04) != 0     // bit 2
	packet.CleanSession = (connectFlags & 0x02) != 0 // bit 1

	// Validate WillQos if WillFlag is set
	if packet.WillFlag && packet.WillQos > 2 {
		return nil, &er.Err{
			Context: "Connect, WillQos",
			Message: er.ErrInvalidWillQos,
		}
	}

	// Parse Keep Alive
	if offset+2 > len(raw) {
		return nil, &er.Err{
			Context: "Connect",
			Message: er.ErrInvalidConnPacket,
		}
	}
	packet.KeepAlive = binary.BigEndian.Uint16(raw[offset : offset+2])
	offset += 2

	clientIDLen := binary.BigEndian.Uint16(raw[offset : offset+2])
	offset += 2

	if offset+int(clientIDLen) > len(raw) {
		return nil, &er.Err{
			Context: "Connect",
			Message: er.ErrInvalidConnPacket,
		}
	}
	packet.ClientID = string(raw[offset : offset+int(clientIDLen)])
	offset += int(clientIDLen)

	cErr := packet.ValidateClientID()
	if cErr != nil {
		if errors.Is(cErr, er.ErrEmptyClientID) {
			// If Client ID is not set from client
			// We assign a uuid to the Client ID from the server
			packet.ClientID = uuid.NewString()
		} else if errors.Is(cErr, er.ErrEmptyAndCleanSessionClientID) {
			// Client must set clean session to 1
			return nil, &er.Err{
				Context: "Connect, ClientID",
				Message: er.ErrIdentifierRejected,
			}
		} else {
			// Bubble it up
			return nil, cErr
		}
	}

	// Parse WillTopic & WillMessage if Will is WillFlag is set
	if packet.WillFlag {
		if offset+2 > len(raw) {
			return nil, &er.Err{
				Context: "Connect, WillFlag",
				Message: er.ErrInvalidConnPacket,
			}
		}
		willTopicLen := binary.BigEndian.Uint16(raw[offset : offset+2])
		offset += 2
		if offset+int(willTopicLen) > len(raw) {
			return nil, &er.Err{
				Context: "Connect, WillTopic",
				Message: er.ErrInvalidConnPacket,
			}
		}
		packet.WillTopic = string(raw[offset : offset+int(willTopicLen)])
		offset += int(willTopicLen)
		if offset+2 > len(raw) {
			return nil, &er.Err{
				Context: "Connect, WillTopic",
				Message: er.ErrInvalidConnPacket,
			}
		}

		willMessageLen := binary.BigEndian.Uint16(raw[offset : offset+2])
		offset += 2
		if offset+int(willMessageLen) > len(raw) {
			return nil, &er.Err{
				Context: "Connect, WillMessage",
				Message: er.ErrInvalidConnPacket,
			}
		}
		packet.WillMessage = string(raw[offset : offset+int(willMessageLen)])
		offset += int(willMessageLen)
	}

	// Username/Password dependency check
	if !packet.UsernameFlag && packet.PasswordFlag {
		return nil, &er.Err{
			Context: "Connect, UsernameFlag + PasswordFlag",
			Message: er.ErrPasswordWithoutUsername,
		}
	}

	// Parse Username if UsernameFlag is set
	if packet.UsernameFlag {
		if offset+2 > len(raw) {
			return nil, &er.Err{
				Context: "Connect, UsernameFlag",
				Message: er.ErrMalformedUsernameField,
			}
		}

		usernameLen := binary.BigEndian.Uint16(raw[offset : offset+2])
		offset += 2

		if offset+int(usernameLen) > len(raw) {
			return nil, &er.Err{
				Context: "Connect, Username",
				Message: er.ErrMalformedUsernameField,
			}
		}
		packet.Username = string(raw[offset : offset+int(usernameLen)])
		offset += int(usernameLen)
	}

	// Parse Password if PasswordFlag is set
	if packet.PasswordFlag {
		if offset+2 > len(raw) {
			return nil, &er.Err{
				Context: "Connect, PasswordFlag",
				Message: er.ErrMalformedPasswordField,
			}
		}

		passwordLen := binary.BigEndian.Uint16(raw[offset : offset+2])
		offset += 2

		if offset+int(passwordLen) > len(raw) {
			return nil, &er.Err{
				Context: "Connect, Password",
				Message: er.ErrMalformedPasswordField,
			}
		}
		packet.Password = string(raw[offset : offset+int(passwordLen)])
	}

	return packet, nil
}

func (c *ConnectPacket) ValidateClientID() error {
	// Check if ClientID is empty (zero bytes)
	if len(c.ClientID) == 0 {
		// Empty ClientID is allowed only if CleanSession is set to 1
		if !c.CleanSession {
			return &er.Err{
				Context: "Connect, ClientID",
				Message: er.ErrEmptyAndCleanSessionClientID,
			}
		}
		return &er.Err{
			Context: "Connect, ClientID",
			Message: er.ErrEmptyClientID,
		}
	}

	// Check ClientID length (1-23 UTF-8 encoded bytes)
	if len(c.ClientID) > 23 {
		return &er.Err{
			Context: "Connect, ClientID",
			Message: er.ErrClientIDLengthExceed,
		}
	}

	// Check allowed characters: 0-9, a-z, A-Z
	allowedChars := "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	for _, char := range c.ClientID {
		if !strings.ContainsRune(allowedChars, char) {
			return &er.Err{
				Context: "Connect, ClientID",
				Message: er.ErrInvalidCharsClientID,
			}
		}
	}

	return nil
}
